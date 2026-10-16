package sessions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
)

const (
	speedMetadataBlock     = 64 << 10
	speedMetadataLineLimit = 1 << 20
)

var errSpeedMetadataLineTooLong = errors.New("speed metadata line exceeds limit")
var errInvalidSpeedMetadata = errors.New("invalid speed metadata")

// restoreSpeedContext finds the latest model and tier before the tail without
// replaying old requests or parsing historical tool output.
func restoreSpeedContext(ctx context.Context, r io.ReaderAt, end int64, c *speedCursor) error {
	wantContext, wantTier := true, true
	prefixEnd := end
	overlap := max(len(markTurnContext), len(markSettings)) - 1
	var suffix []byte
	block := make([]byte, speedMetadataBlock+overlap)
	for end > 0 && (wantContext || wantTier) {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := max(end-speedMetadataBlock, 0)
		n := int(end - start)
		buf := block[:n+len(suffix)]
		if _, err := r.ReadAt(buf[:n], start); err != nil {
			return err
		}
		copy(buf[n:], suffix)
		for searchEnd := len(buf); searchEnd > 0 && (wantContext || wantTier); {
			i, kind := latestSpeedMetadata(buf[:searchEnd], wantContext, wantTier)
			if i < 0 {
				break
			}
			if i >= n {
				searchEnd = i
				continue
			}
			line, err := readSpeedMetadataLine(r, start+int64(i), prefixEnd)
			if err != nil {
				return err
			}
			matched, err := applySpeedMetadata(line, kind, c)
			if err != nil {
				return err
			}
			if matched && kind == "context" {
				wantContext = false
			} else if matched {
				wantTier = false
			}
			searchEnd = i
		}
		keep := min(overlap, n)
		suffix = append(suffix[:0], buf[:keep]...)
		end = start
	}
	return nil
}

func applySpeedMetadata(line []byte, kind string, c *speedCursor) (bool, error) {
	var l speedLine
	if json.Unmarshal(line, &l) != nil {
		return false, errInvalidSpeedMetadata
	}
	switch kind {
	case "context":
		if l.Type != "turn_context" {
			return false, nil
		}
		c.model, c.effort = l.Payload.Model, l.Payload.Effort
	case "tier":
		if l.Type != "event_msg" || l.Payload.Type != "thread_settings_applied" {
			return false, nil
		}
		c.tier = codexTier(l.Payload.ThreadSettings.ServiceTier)
	}
	return true, nil
}

func latestSpeedMetadata(data []byte, wantContext, wantTier bool) (int, string) {
	i, kind := -1, ""
	if wantContext {
		i, kind = bytes.LastIndex(data, markTurnContext), "context"
	}
	if wantTier {
		if j := bytes.LastIndex(data, markSettings); j > i {
			i, kind = j, "tier"
		}
	}
	return i, kind
}

func readSpeedMetadataLine(r io.ReaderAt, marker, end int64) ([]byte, error) {
	start, ok, err := findSpeedLineStart(r, marker)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errSpeedMetadataLineTooLong
	}
	lineEnd, ok, err := findSpeedLineEnd(r, marker, min(end, start+speedMetadataLineLimit+1))
	if err != nil {
		return nil, err
	}
	if !ok || lineEnd-start > speedMetadataLineLimit {
		return nil, errSpeedMetadataLineTooLong
	}
	line := make([]byte, lineEnd-start)
	_, err = r.ReadAt(line, start)
	return line, err
}

func findSpeedLineStart(r io.ReaderAt, marker int64) (int64, bool, error) {
	limit := max(marker-speedMetadataLineLimit, 0)
	buf := make([]byte, speedMetadataBlock)
	for pos := marker; pos > limit; {
		start := max(pos-speedMetadataBlock, limit)
		n := int(pos - start)
		if _, err := r.ReadAt(buf[:n], start); err != nil {
			return 0, false, err
		}
		if i := bytes.LastIndexByte(buf[:n], '\n'); i >= 0 {
			return start + int64(i) + 1, true, nil
		}
		pos = start
	}
	return 0, limit == 0, nil
}

func findSpeedLineEnd(r io.ReaderAt, marker, end int64) (int64, bool, error) {
	buf := make([]byte, speedMetadataBlock)
	for pos := marker; pos < end; {
		n := int(min(int64(len(buf)), end-pos))
		if _, err := r.ReadAt(buf[:n], pos); err != nil {
			return 0, false, err
		}
		if i := bytes.IndexByte(buf[:n], '\n'); i >= 0 {
			return pos + int64(i), true, nil
		}
		pos += int64(n)
	}
	return end, true, nil
}
