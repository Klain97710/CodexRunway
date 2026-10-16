package speed

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadTailShortFileIsWhole(t *testing.T) {
	path := writeTemp(t, "one\ntwo\n")
	data, truncated, err := ReadTail(path, 100)
	if err != nil || truncated || string(data) != "one\ntwo\n" {
		t.Fatalf("data = %q, truncated = %v, err = %v", data, truncated, err)
	}
}

func TestReadTailEmptyRegularFile(t *testing.T) {
	data, truncated, err := ReadTail(writeTemp(t, ""), 100)
	if err != nil || truncated || len(data) != 0 {
		t.Fatalf("data = %q, truncated = %v, err = %v", data, truncated, err)
	}
}

func TestReadTailRejectsNonRegularFiles(t *testing.T) {
	for name, path := range map[string]string{"directory": t.TempDir(), "device": os.DevNull} {
		t.Run(name, func(t *testing.T) {
			data, truncated, err := ReadTail(path, 100)
			if err == nil || os.IsNotExist(err) || len(data) != 0 || truncated {
				t.Fatalf("invalid log must fail explicitly: data = %q, truncated = %v, err = %v", data, truncated, err)
			}
		})
	}
}

func TestReadTailDropsTheCutLine(t *testing.T) {
	path := writeTemp(t, "first line\nsecond\nthird\n")
	data, truncated, err := ReadTail(path, 10) // starts inside "second"
	if err != nil || !truncated || string(data) != "third\n" {
		t.Fatalf("data = %q, truncated = %v, err = %v", data, truncated, err)
	}
	// A cut exactly at a line start keeps that line.
	data, truncated, err = ReadTail(path, 13)
	if err != nil || !truncated || string(data) != "second\nthird\n" {
		t.Fatalf("data = %q, truncated = %v, err = %v", data, truncated, err)
	}
}

func TestReadTailSingleLongLine(t *testing.T) {
	path := writeTemp(t, strings.Repeat("x", 50))
	data, truncated, err := ReadTail(path, 10)
	if err != nil || !truncated || len(data) != 0 {
		t.Fatalf("data = %q, truncated = %v, err = %v", data, truncated, err)
	}
}

func TestReadTailAtUsesGivenSnapshot(t *testing.T) {
	path := writeTemp(t, "one\ntwo\n")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("future\n"); err != nil {
		t.Fatal(err)
	}
	data, start, truncated, err := ReadTailAt(f, info.Size(), 100)
	if err != nil || start != 0 || truncated || string(data) != "one\ntwo\n" {
		t.Fatalf("data = %q, start = %d, truncated = %v, err = %v", data, start, truncated, err)
	}
}

func TestReadTailAtRejectsShortSnapshot(t *testing.T) {
	path := writeTemp(t, "one\ntwo\n")
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(4); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadTailAt(f, info.Size(), 100); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v", err)
	}
}

func TestReadTailMissingFile(t *testing.T) {
	if _, _, err := ReadTail(filepath.Join(t.TempDir(), "none"), 10); !os.IsNotExist(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestEachLineSkipsEmptyAndTrimsCR(t *testing.T) {
	var got []string
	err := EachLine(context.Background(), []byte("a\r\n\nb\n\r\nc"), func(line []byte) { got = append(got, string(line)) })
	if err != nil || strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("lines = %v, err = %v", got, err)
	}
}

func TestEachLineHonoursCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := EachLine(ctx, []byte("a\nb\n"), func([]byte) { calls++ })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}
