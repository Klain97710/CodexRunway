package usage

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
)

// DuplicateConflictError reports two files with the same name but different
// content under the session roots, so there is no single truth to index.
type DuplicateConflictError struct{ Basename string }

func (e *DuplicateConflictError) Error() string {
	return "usage: duplicate session files with different content: " + e.Basename
}

// indexRefresher brings the index in line with the files on disk.
type indexRefresher struct {
	codexHome string
	store     *indexStore
	indexer   *sourceIndexer
	diag      *diagState
}

type selectedSource struct {
	file            sourceFile
	byteHash        []byte
	indexedByteHash []byte
}

type sourceSelection struct {
	sources          []selectedSource
	hashCacheRows    []cachedFullHash
	hashCacheChanged bool
}

// refresh updates every source and returns the warnings describing anomalies.
func (r *indexRefresher) refresh(ctx context.Context, progress *ProgressReporter) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.diag.update(func(d *Diagnostics) { d.IndexPasses++ })
	indexed, err := r.store.sourceRows(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]indexedSource, len(indexed))
	for _, s := range indexed {
		byName[s.Basename] = s
	}
	files, err := listSources(ctx, r.codexHome)
	if err != nil {
		return nil, err
	}
	sel, err := r.selectSources(ctx, files, byName)
	if err != nil {
		return nil, err
	}
	sources := append([]selectedSource{}, sel.sources...)
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].file.Basename < sources[j].file.Basename })
	total := len(sources)
	if total == 0 {
		progress.Report(ProgressRefreshingIndex(0, 0, ""), true)
	}
	var provisional anomalies
	for i, item := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress.Report(ProgressRefreshingIndex(i, total, item.file.Basename), i == 0 || i+1 == total)
		var existing *indexedSource
		if e, ok := byName[item.file.Basename]; ok {
			existing = &e
		}
		a, err := r.update(ctx, item, existing)
		if err != nil {
			return nil, err
		}
		if provisional, err = provisional.add(a); err != nil {
			return nil, err
		}
	}
	if total > 0 {
		progress.Report(ProgressRefreshingIndex(total, total, ""), true)
	}
	if sel.hashCacheChanged {
		if err := r.store.replaceHashCache(ctx, sel.hashCacheRows); err != nil {
			return nil, err
		}
	}
	retained := make(map[string]struct{}, len(sel.sources))
	for _, s := range sel.sources {
		retained[s.file.Basename] = struct{}{}
	}
	removed, err := r.store.removeSources(ctx, retained)
	if err != nil {
		return nil, err
	}
	r.diag.update(func(d *Diagnostics) { d.RemovedFiles += removed })
	return r.anomalyWarnings(ctx, provisional)
}

func (r *indexRefresher) selectSources(ctx context.Context, files []sourceFile, byName map[string]indexedSource) (sourceSelection, error) {
	cachedRows, err := r.store.hashCacheRows(ctx)
	if err != nil {
		return sourceSelection{}, err
	}
	cachedByID := make(map[sourceIdentity]cachedFullHash, len(cachedRows))
	for _, c := range cachedRows {
		cachedByID[c.Identity] = c
	}
	groups := map[string][]sourceFile{}
	for _, f := range files {
		groups[f.Basename] = append(groups[f.Basename], f)
	}
	names := make([]string, 0, len(groups))
	for n := range groups {
		names = append(names, n)
	}
	sort.Strings(names)

	var selected []selectedSource
	retainedHashes := map[sourceIdentity]cachedFullHash{}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return sourceSelection{}, err
		}
		candidates := groups[name]
		indexed, hasIndexed := byName[name]
		var indexedHash []byte
		if hasIndexed {
			indexedHash = cachedByteHash(indexed, cachedByID)
		}
		if len(candidates) == 1 {
			f := candidates[0]
			var digest []byte
			if c, ok := cachedByID[identityOf(f)]; ok && c.matches(f) {
				retainedHashes[c.Identity] = c
				digest = c.Digest
			}
			selected = append(selected, selectedSource{file: f, byteHash: digest, indexedByteHash: indexedHash})
			continue
		}
		type fileDigest struct {
			file   sourceFile
			digest []byte
		}
		var hashes []fileDigest
		for _, f := range candidates {
			id := identityOf(f)
			row, ok := retainedHashes[id]
			if !ok {
				if c, hit := cachedByID[id]; hit && c.matches(f) {
					row, ok = c, true
				}
			}
			if !ok {
				h, err := hashFull(ctx, f)
				if err != nil {
					return sourceSelection{}, err
				}
				r.diag.update(func(d *Diagnostics) { d.ValidationBytesRead += int(h.BytesRead) })
				row = cachedHashOf(f, append([]byte{}, h.Digest[:]...))
			}
			retainedHashes[id] = row
			hashes = append(hashes, fileDigest{f, row.Digest})
		}
		for _, h := range hashes[1:] {
			if !bytes.Equal(h.digest, hashes[0].digest) {
				return sourceSelection{}, &DuplicateConflictError{Basename: name}
			}
		}
		r.diag.update(func(d *Diagnostics) { d.DuplicateFiles += len(candidates) - 1 })
		preferred := hashes[0].file
		if hasIndexed {
			for _, h := range hashes {
				if sameIdentity(h.file, indexed) {
					preferred = h.file
					break
				}
			}
		}
		selected = append(selected, selectedSource{file: preferred, byteHash: hashes[0].digest, indexedByteHash: indexedHash})
	}
	rows := make([]cachedFullHash, 0, len(retainedHashes))
	for _, c := range retainedHashes {
		rows = append(rows, c)
	}
	return sourceSelection{sources: selected, hashCacheRows: rows, hashCacheChanged: !sameHashSet(rows, cachedRows)}, nil
}

func sameHashSet(a, b []cachedFullHash) bool {
	if len(a) != len(b) {
		return false
	}
	index := make(map[string]struct{}, len(a))
	for _, c := range a {
		index[hashKey(c)] = struct{}{}
	}
	for _, c := range b {
		if _, ok := index[hashKey(c)]; !ok {
			return false
		}
	}
	return true
}

func hashKey(c cachedFullHash) string {
	return fmt.Sprintf("%d/%d/%d/%d/%d/%d/%x", c.Identity.Device, c.Identity.Inode, c.Identity.BirthNS, c.Size, c.MtimeNS, c.CtimeNS, c.Digest)
}

func cachedByteHash(indexed indexedSource, cache map[sourceIdentity]cachedFullHash) []byte {
	c, ok := cache[sourceIdentity{indexed.Device, indexed.Inode, indexed.BirthNS}]
	if !ok || c.Size != indexed.Size || c.MtimeNS != indexed.MtimeNS || c.CtimeNS != indexed.CtimeNS {
		return nil
	}
	return c.Digest
}

func sameIdentity(f sourceFile, s indexedSource) bool {
	return f.meta.Device == s.Device && f.meta.Inode == s.Inode && f.meta.BirthNS == s.BirthNS
}

func (r *indexRefresher) update(ctx context.Context, sel selectedSource, existing *indexedSource) (anomalies, error) {
	if existing == nil {
		return r.indexer.rebuild(ctx, sel.file, nil)
	}
	if sameIdentity(sel.file, *existing) {
		return r.updateSameIdentity(ctx, sel, *existing)
	}
	if canAdoptExactCopy(sel, *existing) {
		if err := r.indexer.adopt(ctx, sel.file, *existing); err != nil {
			return anomalies{}, err
		}
		r.diag.update(func(d *Diagnostics) { d.AdoptedFiles++ })
		return anomalies{}, nil
	}
	return r.indexer.rebuild(ctx, sel.file, existing)
}

func (r *indexRefresher) updateSameIdentity(ctx context.Context, sel selectedSource, e indexedSource) (anomalies, error) {
	f := sel.file
	sameSize := f.meta.Size == e.Size
	pathChanged := f.Path != e.Path || string(f.Root) != e.Root
	sameMtime := f.meta.MtimeNS == e.MtimeNS
	if pathChanged && sameSize && sameMtime && e.CompleteOffset == e.Size {
		ok, err := r.canAdoptMove(ctx, sel, e)
		if err != nil {
			return anomalies{}, err
		}
		if ok {
			if err := r.indexer.adopt(ctx, f, e); err != nil {
				return anomalies{}, err
			}
			r.diag.update(func(d *Diagnostics) { d.AdoptedFiles++ })
			return anomalies{}, nil
		}
		return r.indexer.rebuild(ctx, f, &e)
	}
	sameTimes := sameMtime && f.meta.CtimeNS == e.CtimeNS
	if sameSize && sameTimes && e.CompleteOffset == e.Size {
		r.diag.update(func(d *Diagnostics) { d.CacheHits++ })
		return anomalies{}, nil
	}
	mayAppend := f.meta.Size > e.Size || (sameSize && sameTimes && e.CompleteOffset < e.Size)
	if mayAppend {
		ok, err := r.indexer.canAppend(ctx, f, e)
		if err != nil {
			return anomalies{}, err
		}
		if ok {
			return r.indexer.append(ctx, f, e)
		}
	}
	return r.indexer.rebuild(ctx, f, &e)
}

func (r *indexRefresher) canAdoptMove(ctx context.Context, sel selectedSource, e indexedSource) (bool, error) {
	if sel.file.meta.Size != e.Size || e.CompleteOffset != e.Size {
		return false, nil
	}
	if sel.byteHash != nil && sel.indexedByteHash != nil {
		return bytes.Equal(sel.byteHash, sel.indexedByteHash), nil
	}
	return r.indexer.canAdopt(ctx, sel.file, e)
}

func canAdoptExactCopy(sel selectedSource, e indexedSource) bool {
	if sel.file.meta.Size != e.Size || e.CompleteOffset != e.Size || sel.byteHash == nil || sel.indexedByteHash == nil {
		return false
	}
	return bytes.Equal(sel.byteHash, sel.indexedByteHash)
}

func (r *indexRefresher) anomalyWarnings(ctx context.Context, provisional anomalies) ([]string, error) {
	rows, err := r.store.sourceRows(ctx)
	if err != nil {
		return nil, err
	}
	malformed, oversized := provisional.Malformed, provisional.Oversized
	for _, row := range rows {
		if malformed, err = checkedAdd(malformed, row.MalformedLines, "malformed line count"); err != nil {
			return nil, err
		}
		if oversized, err = checkedAdd(oversized, row.OversizedLines, "oversized line count"); err != nil {
			return nil, err
		}
	}
	var warnings []string
	if malformed > 0 {
		warnings = append(warnings, "malformed-jsonl-lines:"+strconv.FormatInt(malformed, 10))
	}
	if oversized > 0 {
		warnings = append(warnings, "oversized-jsonl-lines:"+strconv.FormatInt(oversized, 10))
	}
	return warnings, nil
}
