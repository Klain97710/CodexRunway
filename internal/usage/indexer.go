package usage

import (
	"bytes"
	"context"
	"errors"
)

// sourceIndexer turns one log file into rows of the index.
type sourceIndexer struct {
	store         *indexStore
	parserVersion int64
	stream        *logStream
	diag          *diagState
}

func newSourceIndexer(store *indexStore, parserVersion int64, stream *logStream, diag *diagState) *sourceIndexer {
	return &sourceIndexer{store: store, parserVersion: parserVersion, stream: stream, diag: diag}
}

func (x *sourceIndexer) initialSource(f sourceFile) indexedSource {
	return indexedSource{
		Basename: f.Basename, Root: string(f.Root), Path: f.Path,
		Device: f.meta.Device, Inode: f.meta.Inode, BirthNS: f.meta.BirthNS,
		MtimeNS: f.meta.MtimeNS, CtimeNS: f.meta.CtimeNS, Size: f.meta.Size,
		CurrentModel: unknownModel, CurrentProject: UnknownProject,
		FirstHash: emptyHash, CheckpointHash: emptyHash, ParserVersion: x.parserVersion,
	}
}

// rebuild indexes the whole file, replacing whatever was stored for it.
func (x *sourceIndexer) rebuild(ctx context.Context, f sourceFile, existing *indexedSource) (anomalies, error) {
	src, events, metrics, err := x.scan(ctx, f, 0, unknownModel, UnknownProject, nil)
	if err != nil {
		return anomalies{}, err
	}
	if _, err := x.store.replaceSource(ctx, src, 0, events); err != nil {
		return anomalies{}, err
	}
	x.diag.update(func(d *Diagnostics) {
		metrics.record(d)
		d.RebuiltFiles++
	})
	return metrics.provisional(), nil
}

// append indexes only what was written after the stored checkpoint.
func (x *sourceIndexer) append(ctx context.Context, f sourceFile, existing indexedSource) (anomalies, error) {
	src, events, metrics, err := x.scan(ctx, f, existing.CompleteOffset, existing.CurrentModel, existing.CurrentProject, &existing)
	if err != nil {
		return anomalies{}, err
	}
	if _, err := x.store.replaceSource(ctx, src, existing.CompleteOffset, events); err != nil {
		return anomalies{}, err
	}
	x.diag.update(func(d *Diagnostics) {
		metrics.record(d)
		d.AppendedFiles++
	})
	return metrics.provisional(), nil
}

// adopt records a moved or copied file under the existing row.
func (x *sourceIndexer) adopt(ctx context.Context, f sourceFile, existing indexedSource) error {
	adopted := existing
	adopted.applyMetadata(f)
	return x.store.adoptSource(ctx, adopted)
}

// canAdopt re-reads the whole file and checks it is byte-for-byte what the
// row was built from.
func (x *sourceIndexer) canAdopt(ctx context.Context, f sourceFile, existing indexedSource) (bool, error) {
	if f.meta.Size != existing.Size || existing.CompleteOffset != existing.Size || existing.ContentFingerprint == nil {
		return false, nil
	}
	res, err := x.stream.read(ctx, readRequest{Path: f.Path, Expected: &f, RequireStable: true},
		func(parsedLine) error { return nil })
	if errors.Is(err, ErrSourceMetadataChanged) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	x.diag.update(func(d *Diagnostics) {
		d.ValidationBytesRead += res.BytesRead
		d.MaxBufferedBytes = max(d.MaxBufferedBytes, res.MaxBufferedBytes)
	})
	malformed := int64(res.MalformedLines - res.IncompleteMalformed)
	oversized := int64(res.OversizedLines - res.IncompleteOversized)
	return res.Meta.Size == f.meta.Size &&
		res.LastCompleteOffset == res.Meta.Size &&
		bytes.Equal(res.ContentFingerprint, existing.ContentFingerprint) &&
		malformed == existing.MalformedLines &&
		oversized == existing.OversizedLines, nil
}

// canAppend checks that the bytes before the stored checkpoint are still the
// ones that were indexed, by comparing the first and last hash windows.
func (x *sourceIndexer) canAppend(ctx context.Context, f sourceFile, e indexedSource) (bool, error) {
	if e.CompleteOffset > f.meta.Size || e.ContentFingerprint == nil ||
		e.FirstHashLength < 0 || e.CheckpointHashLength < 0 ||
		uint64(e.FirstHashLength) > f.meta.Size || uint64(e.CheckpointHashLength) > e.CompleteOffset {
		return false, nil
	}
	ok, err := x.validate(ctx, f, 0, uint64(e.FirstHashLength), e.FirstHash)
	if err != nil || !ok {
		return false, err
	}
	n := uint64(e.CheckpointHashLength)
	return x.validate(ctx, f, e.CompleteOffset-n, e.CompleteOffset, e.CheckpointHash)
}

func (x *sourceIndexer) validate(ctx context.Context, f sourceFile, lo, hi uint64, want []byte) (bool, error) {
	h, err := hashRange(ctx, f, lo, hi)
	if err != nil {
		return false, err
	}
	x.diag.update(func(d *Diagnostics) { d.ValidationBytesRead += int(h.BytesRead) })
	return bytes.Equal(h.Digest[:], want), nil
}

// scan streams the file from offset and returns the row to store and the
// events found. Nothing is written to the database.
func (x *sourceIndexer) scan(ctx context.Context, f sourceFile, offset uint64, model, project string, existing *indexedSource) (indexedSource, []indexedEvent, scanMetrics, error) {
	curModel, curProject := model, project
	cpModel, cpProject := model, project
	var events []indexedEvent
	var initialFP []byte
	if existing != nil {
		initialFP = existing.ContentFingerprint
	}
	res, err := x.stream.read(ctx, readRequest{Path: f.Path, FromOffset: offset, InitialFP: initialFP, Expected: &f},
		func(line parsedLine) error {
			rec := line.Record
			if rec.SessionCWD != nil {
				curProject = ProjectDisplayName(*rec.SessionCWD)
			}
			if rec.ContextModel != nil {
				curModel = *rec.ContextModel
			}
			eventModel := curModel
			if rec.Model != nil {
				eventModel = *rec.Model
			}
			var turns int64
			if rec.CountsAsTurn {
				turns = 1
			}
			switch {
			case rec.Usage != nil:
				cached := max(0, rec.Usage.CachedInputTokens)
				uncached := int64(0)
				if rec.Usage.InputTokens >= cached {
					uncached = rec.Usage.InputTokens - cached
				}
				events = append(events, indexedEvent{
					ByteOffset: line.Offset, Timestamp: rec.Timestamp, DayKey: rec.DayKey,
					Model: eventModel, Project: curProject,
					Uncached: uncached, Cached: cached, Output: max(0, rec.Usage.OutputTokens), Turns: turns,
				})
			case rec.CountsAsTurn:
				events = append(events, indexedEvent{
					ByteOffset: line.Offset, Timestamp: rec.Timestamp, DayKey: rec.DayKey,
					Model: eventModel, Project: curProject, Turns: 1,
				})
			}
			if line.LFComplete {
				cpModel, cpProject = curModel, curProject
			}
			return nil
		})
	if err != nil {
		return indexedSource{}, nil, scanMetrics{}, err
	}
	first, checkpoint, hashBytes, err := x.hashes(ctx, f, res.Meta.Size, res.LastCompleteOffset, existing)
	if err != nil {
		return indexedSource{}, nil, scanMetrics{}, err
	}
	src := x.initialSource(f)
	if existing != nil {
		src.ID = existing.ID
	}
	src.Device, src.Inode, src.BirthNS = res.Meta.Device, res.Meta.Inode, res.Meta.BirthNS
	src.MtimeNS, src.CtimeNS, src.Size = res.Meta.MtimeNS, res.Meta.CtimeNS, res.Meta.Size
	src.CompleteOffset = res.LastCompleteOffset
	src.CurrentModel, src.CurrentProject = cpModel, cpProject
	src.FirstHash, src.FirstHashLength = first.digest, first.length
	src.CheckpointHash, src.CheckpointHashLength = checkpoint.digest, checkpoint.length
	malformed := int64(res.MalformedLines - res.IncompleteMalformed)
	oversized := int64(res.OversizedLines - res.IncompleteOversized)
	var prevMalformed, prevOversized int64
	if existing != nil {
		prevMalformed, prevOversized = existing.MalformedLines, existing.OversizedLines
	}
	if src.MalformedLines, err = checkedAdd(prevMalformed, malformed, "malformed line count"); err != nil {
		return indexedSource{}, nil, scanMetrics{}, err
	}
	if src.OversizedLines, err = checkedAdd(prevOversized, oversized, "oversized line count"); err != nil {
		return indexedSource{}, nil, scanMetrics{}, err
	}
	src.ContentFingerprint = res.ContentFingerprint
	return src, events, scanMetrics{stream: res, hashBytes: hashBytes}, nil
}

type hashValue struct {
	digest []byte
	length int64
}

func (x *sourceIndexer) hashes(ctx context.Context, f sourceFile, snapshotSize, completeOffset uint64, existing *indexedSource) (first, checkpoint hashValue, bytesRead int, err error) {
	firstLen := min(uint64(hashWindowBytes), snapshotSize)
	if existing != nil && uint64(existing.FirstHashLength) == firstLen {
		first = hashValue{existing.FirstHash, int64(firstLen)}
	} else {
		h, err := hashRange(ctx, f, 0, firstLen)
		if err != nil {
			return first, checkpoint, 0, err
		}
		first = hashValue{h.Digest[:], int64(firstLen)}
		bytesRead += int(h.BytesRead)
	}
	cpLen := min(uint64(hashWindowBytes), completeOffset)
	h, err := hashRange(ctx, f, completeOffset-cpLen, completeOffset)
	if err != nil {
		return first, checkpoint, 0, err
	}
	checkpoint = hashValue{h.Digest[:], int64(cpLen)}
	bytesRead += int(h.BytesRead)
	return first, checkpoint, bytesRead, nil
}
