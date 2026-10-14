package usage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// sourceRoot names a top-level directory of the Codex home that holds session
// logs.
type sourceRoot string

const (
	rootSessions sourceRoot = "sessions"
	rootArchived sourceRoot = "archived_sessions"
)

var allSourceRoots = []sourceRoot{rootSessions, rootArchived}

func (r sourceRoot) valid() bool { return r == rootSessions || r == rootArchived }

// sourceFile is one inventoried JSONL log with the stat snapshot taken when it
// was listed.
type sourceFile struct {
	Path     string
	Root     sourceRoot
	Basename string
	meta     fileMeta
}

func (s sourceFile) Size() uint64 { return s.meta.Size }

// ErrCannotEnumerate reports a session root that exists but cannot be listed.
type ErrCannotEnumerate struct {
	Root string
	Err  error
}

func (e *ErrCannotEnumerate) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("usage: cannot enumerate %s: %v", e.Root, e.Err)
	}
	return "usage: cannot enumerate " + e.Root
}
func (e *ErrCannotEnumerate) Unwrap() error { return e.Err }

// listSources returns the regular *.jsonl files below codexHome/sessions and
// codexHome/archived_sessions, sorted by (root, path). Symlinks and other
// special files are ignored.
func listSources(ctx context.Context, codexHome string) ([]sourceFile, error) {
	var out []sourceFile
	for _, root := range allSourceRoots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := listRoot(ctx, filepath.Join(codexHome, string(root)), root)
		if err != nil {
			return nil, err
		}
		out = append(out, files...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Root != out[j].Root {
			return out[i].Root < out[j].Root
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func hasJSONLExtension(name string) bool {
	i := strings.LastIndexByte(name, '.')
	return i > 0 && name[i+1:] == "jsonl"
}

func listRoot(ctx context.Context, dir string, root sourceRoot) ([]sourceFile, error) {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.IsDir() {
		return nil, &ErrCannotEnumerate{Root: string(root), Err: err}
	}
	walkDir := dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		walkDir = resolved
	}
	var out []sourceFile
	err = filepath.WalkDir(walkDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() || !hasJSONLExtension(d.Name()) {
			return nil
		}
		meta, err := lstatMeta(p)
		if err != nil {
			return fmt.Errorf("usage: stat %s: %w", d.Name(), err)
		}
		if !meta.Regular {
			return nil
		}
		// Paths are reported with symlinks in the root resolved (such as
		// /tmp -> /private/tmp), which is what the Swift enumerator stores in
		// the index, so databases stay interchangeable.
		out = append(out, sourceFile{Path: p, Root: root, Basename: d.Name(), meta: meta})
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var ce *ErrCannotEnumerate
		if errors.As(err, &ce) {
			return nil, err
		}
		return nil, &ErrCannotEnumerate{Root: string(root), Err: err}
	}
	return out, nil
}

// fileHash is a SHA-256 digest of a byte range together with how many bytes
// were read to compute it.
type fileHash struct {
	Digest    [sha256.Size]byte
	BytesRead int64
}

const hashChunkSize = 256 * 1024

// hashRange hashes [lo, hi) of the file. It fails if the file at the path is
// not the inventoried one or is shorter than hi.
func hashRange(ctx context.Context, src sourceFile, lo, hi uint64) (fileHash, error) {
	return hashFile(ctx, src, lo, hi, false)
}

// hashFull hashes the whole file and requires its metadata to stay unchanged
// while it is read.
func hashFull(ctx context.Context, src sourceFile) (fileHash, error) {
	return hashFile(ctx, src, 0, 0, true)
}

func hashFile(ctx context.Context, src sourceFile, lo, hi uint64, full bool) (fileHash, error) {
	f, err := os.Open(src.Path)
	if err != nil {
		return fileHash{}, err
	}
	defer f.Close()
	meta, err := fstatMeta(f)
	if err != nil {
		return fileHash{}, err
	}
	if !meta.Regular {
		return fileHash{}, ErrInvalidSourceFile
	}
	if !meta.sameIdentity(src.meta) {
		return fileHash{}, ErrSourceIdentityChanged
	}
	if full && !meta.sameSnapshot(src.meta) {
		return fileHash{}, ErrSourceIdentityChanged
	}
	if full {
		hi = meta.Size
	}
	if hi > meta.Size {
		return fileHash{}, fmt.Errorf("usage: hash range %d beyond snapshot %d", hi, meta.Size)
	}
	if _, err := f.Seek(int64(lo), io.SeekStart); err != nil {
		return fileHash{}, err
	}
	h := sha256.New()
	buf := make([]byte, hashChunkSize)
	pos := lo
	for pos < hi {
		if err := ctx.Err(); err != nil {
			return fileHash{}, err
		}
		want := hi - pos
		if want > uint64(len(buf)) {
			want = uint64(len(buf))
		}
		n, err := f.Read(buf[:want])
		if n == 0 {
			if err == nil || errors.Is(err, io.EOF) {
				return fileHash{}, ErrUnexpectedEOF
			}
			return fileHash{}, err
		}
		h.Write(buf[:n])
		pos += uint64(n)
	}
	if full {
		after, err := fstatMeta(f)
		if err != nil {
			return fileHash{}, err
		}
		if !after.sameSnapshot(meta) {
			return fileHash{}, ErrSourceIdentityChanged
		}
	}
	var out fileHash
	h.Sum(out.Digest[:0])
	out.BytesRead = int64(hi - lo)
	return out, nil
}
