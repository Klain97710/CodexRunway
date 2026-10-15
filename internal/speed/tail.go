package speed

import (
	"bytes"
	"context"
	"io"
	"os"
)

// ReadTail returns the last limit bytes of the file. When the file is longer,
// the cut first line is dropped and truncated is true, so every line returned
// is whole.
func ReadTail(path string, limit int64) (data []byte, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, false, err
	}
	start := max(size-limit, 0)
	// With a cut, one byte more is read to tell a cut at a line start from a
	// cut inside a line.
	from := max(start-1, 0)
	data = make([]byte, size-from)
	n, err := f.ReadAt(data, from)
	if err != nil && err != io.EOF {
		return nil, false, err
	}
	data = data[:n]
	if start == 0 {
		return data, false, nil
	}
	if data[0] == '\n' {
		return data[1:], true, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return data[i+1:], true, nil
	}
	return nil, true, nil // one line longer than the limit
}

// cancelCheckLines is how many lines pass between two checks of the context.
const cancelCheckLines = 512

// EachLine calls fn for every non-empty line of data (without its "\n" or
// "\r"), checking ctx every few hundred lines. fn must not keep the slice.
func EachLine(ctx context.Context, data []byte, fn func(line []byte)) error {
	for n := 0; len(data) > 0; n++ {
		if n%cancelCheckLines == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		line := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		if line = bytes.TrimSuffix(line, []byte("\r")); len(line) > 0 {
			fn(line)
		}
	}
	return ctx.Err()
}
