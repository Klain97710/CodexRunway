package sessions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	managedReadBytes   = 4 << 20
	managedLineBytes   = 1 << 20
	managedTextBytes   = 4 << 10
	managedReturnBytes = 256 << 10
	managedMessageMax  = 200
)

// ReadManagedDetail reads only the files captured by ListManaged. It limits
// disk input, individual records, returned text, and message count.
func ReadManagedDetail(ctx context.Context, session ManagedSession) (ManagedDetail, error) {
	if session.Provider != ProviderCodex && session.Provider != ProviderClaude && session.Provider != ProviderGrok {
		return ManagedDetail{}, fmt.Errorf("unsupported session provider %q", session.Provider)
	}
	detail := ManagedDetail{Session: session, Messages: []ManagedMessage{}}
	state := &managedMessageState{seen: map[string]int{}, chunks: map[string]int{}}
	remaining := int64(managedReadBytes)
	for _, file := range session.Files {
		if err := ctx.Err(); err != nil {
			return ManagedDetail{}, err
		}
		if remaining <= 0 {
			detail.Truncated = true
			break
		}
		if session.Provider == ProviderGrok && filepath.Base(file.Path) == "summary.json" {
			continue
		}
		if err := VerifyManagedFileIdentity(session.Root, file); err != nil {
			return ManagedDetail{}, err
		}
		used, truncated, err := readManagedFile(ctx, session.Provider, file.Path, remaining, &detail, state)
		remaining -= used
		detail.Truncated = detail.Truncated || truncated
		if err != nil {
			return ManagedDetail{}, err
		}
	}
	sort.SliceStable(detail.Messages, func(i, j int) bool {
		if detail.Messages[i].At.IsZero() || detail.Messages[j].At.IsZero() {
			return false
		}
		return detail.Messages[i].At.Before(detail.Messages[j].At)
	})
	return detail, nil
}

func readManagedFile(ctx context.Context, provider, path string, limit int64, detail *ManagedDetail, state *managedMessageState) (int64, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, false, err
	}
	// The file may grow while an active session is displayed, but it may not
	// be replaced between the path check and open.
	matched := false
	for _, scanned := range detail.Session.Files {
		if scanned.Path == path && scanned.Info != nil && os.SameFile(scanned.Info, info) {
			matched = true
			break
		}
	}
	if !matched {
		return 0, false, fmt.Errorf("session file identity changed: %s", filepath.Base(path))
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return 0, false, err
	}
	if !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, info) {
		return 0, false, fmt.Errorf("session file identity changed: %s", filepath.Base(path))
	}
	readLimit := min(limit, info.Size())
	r := bufio.NewReader(io.LimitReader(f, readLimit))
	var used int64
	grokTurn := 0
	truncated := info.Size() > readLimit
	for lineNo := 1; ; lineNo++ {
		if lineNo%256 == 0 {
			if err := ctx.Err(); err != nil {
				return used, truncated, err
			}
		}
		line, n, tooLong, err := readManagedLine(r)
		used += int64(n)
		if tooLong {
			truncated = true
		} else if len(strings.TrimSpace(string(line))) > 0 {
			var obj map[string]any
			dec := json.NewDecoder(strings.NewReader(string(line)))
			dec.UseNumber()
			if decodeErr := dec.Decode(&obj); decodeErr != nil {
				if err == io.EOF && truncated {
					return used, true, nil
				}
				return used, truncated, fmt.Errorf("malformed session record in %s at line %d", path, lineNo)
			}
			message := managedMessage(provider, obj)
			if message.boundary {
				grokTurn++
			} else if message.role != "" && strings.TrimSpace(message.text) != "" {
				if message.chunk {
					message.key = path + "\x00" + strconv.Itoa(grokTurn) + "\x00" + message.key
				}
				if addManagedMessage(detail, state, message) {
					truncated = true
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return used, truncated, nil
			}
			return used, truncated, err
		}
	}
}

func readManagedLine(r *bufio.Reader) ([]byte, int, bool, error) {
	var out []byte
	total := 0
	tooLong := false
	for {
		chunk, err := r.ReadSlice('\n')
		total += len(chunk)
		if !tooLong && len(out)+len(chunk) <= managedLineBytes {
			out = append(out, chunk...)
		} else {
			tooLong = true
			out = nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return out, total, tooLong, err
	}
}

type parsedManagedMessage struct {
	role, text, key, source string
	at                      time.Time
	chunk, boundary         bool
}

type managedMessageState struct {
	seen    map[string]int
	chunks  map[string]int
	sources []string
}

func addManagedMessage(detail *ManagedDetail, state *managedMessageState, message parsedManagedMessage) bool {
	text := message.text
	if !message.chunk {
		text = strings.TrimSpace(text)
	}
	text, cut := truncateManagedText(text, managedTextBytes)
	if message.chunk {
		if i, ok := state.chunks[message.key]; ok {
			detail.Messages[i].Text, cut = truncateManagedText(detail.Messages[i].Text+text, managedTextBytes)
			return cut
		}
	} else if message.key != "" {
		if i, ok := state.seen[message.key]; ok {
			if len(text) > len(detail.Messages[i].Text) {
				detail.Messages[i] = ManagedMessage{Role: message.role, Text: text, At: message.at}
			}
			return cut
		}
	}
	if codexMirror(detail, state, message, text) {
		return cut
	}
	if len(detail.Messages) >= managedMessageMax || managedDetailBytes(detail.Messages)+len(text) > managedReturnBytes {
		return true
	}
	index := len(detail.Messages)
	if message.chunk {
		state.chunks[message.key] = index
	} else if message.key != "" {
		state.seen[message.key] = index
	}
	detail.Messages = append(detail.Messages, ManagedMessage{Role: message.role, Text: text, At: message.at})
	state.sources = append(state.sources, message.source)
	return cut
}

func codexMirror(detail *ManagedDetail, state *managedMessageState, message parsedManagedMessage, text string) bool {
	if len(detail.Messages) == 0 || len(state.sources) != len(detail.Messages) {
		return false
	}
	last := detail.Messages[len(detail.Messages)-1]
	previous := state.sources[len(state.sources)-1]
	pair := (previous == "codex-response" && message.source == "codex-event") ||
		(previous == "codex-event" && message.source == "codex-response")
	near := !last.At.IsZero() && !message.at.IsZero() && last.At.Sub(message.at).Abs() <= 5*time.Second
	return pair && near && last.Role == message.role && last.Text == text
}

func managedDetailBytes(messages []ManagedMessage) int {
	total := 0
	for _, message := range messages {
		total += len(message.Text)
	}
	return total
}

func truncateManagedText(text string, maxBytes int) (string, bool) {
	if len(text) <= maxBytes {
		return text, false
	}
	text = text[:maxBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + "…", true
}
