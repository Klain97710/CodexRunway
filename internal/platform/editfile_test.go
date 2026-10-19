package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func editPolicy(dir string) BackupPolicy {
	return BackupPolicy{Dir: filepath.Join(dir, "backups"), Prefix: "cfg-", Ext: ".txt", Keep: 3}
}

func backupNames(t *testing.T, p BackupPolicy) []string {
	t.Helper()
	entries, err := os.ReadDir(p.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestEditFileWithBackupCreatesNewFileWithoutBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "cfg.txt")
	p := editPolicy(dir)
	err := EditFileWithBackup(path, p, time.Unix(1_800_000_000, 0), func(old []byte) ([]byte, error) {
		if old != nil {
			t.Fatalf("old = %q, want nil", old)
		}
		return []byte("new"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("content = %q", got)
	}
	if names := backupNames(t, p); len(names) != 0 {
		t.Fatalf("backups = %v, want none for a new file", names)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestEditFileWithBackupUnchangedAndErrorsWriteNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := editPolicy(dir)
	now := time.Unix(1_800_000_000, 0)
	if err := EditFileWithBackup(path, p, now, func(old []byte) ([]byte, error) { return old, nil }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := EditFileWithBackup(path, p, now, func([]byte) ([]byte, error) { return []byte("x"), boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "same" {
		t.Fatalf("content = %q", got)
	}
	if names := backupNames(t, p); len(names) != 0 {
		t.Fatalf("backups = %v, want none", names)
	}
}

func TestEditFileWithBackupKeepsModeAndRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("v0"), 0o640); err != nil {
		t.Fatal(err)
	}
	p := editPolicy(dir)
	base := time.Unix(1_800_000_000, 0)
	for i := 1; i <= 5; i++ {
		content := []byte{'v', byte('0' + i)}
		err := EditFileWithBackup(path, p, base.Add(time.Duration(i)*time.Second), func([]byte) ([]byte, error) {
			return content, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	names := backupNames(t, p)
	if len(names) != 3 {
		t.Fatalf("backups = %v, want 3", names)
	}
	// The newest backup is the pre-edit content of the last edit (v4).
	last, _ := os.ReadFile(filepath.Join(p.Dir, names[len(names)-1]))
	first, _ := os.ReadFile(filepath.Join(p.Dir, names[0]))
	if string(last) != "v4" || string(first) != "v2" {
		t.Fatalf("kept backups = %q..%q, want v2..v4", first, last)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("file mode = %v, want 0640 preserved", info.Mode().Perm())
		}
		bi, _ := os.Stat(filepath.Join(p.Dir, names[0]))
		di, _ := os.Stat(p.Dir)
		if bi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
			t.Fatalf("backup mode = %v dir = %v", bi.Mode().Perm(), di.Mode().Perm())
		}
	}
}

func TestEditFileWithBackupSameInstantDoesNotOverwriteBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("orig"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := editPolicy(dir)
	now := time.Unix(1_800_000_000, 0)
	for _, content := range []string{"one", "two"} {
		if err := EditFileWithBackup(path, p, now, func([]byte) ([]byte, error) { return []byte(content), nil }); err != nil {
			t.Fatal(err)
		}
	}
	names := backupNames(t, p)
	if len(names) != 2 {
		t.Fatalf("backups = %v, want 2", names)
	}
	a, _ := os.ReadFile(filepath.Join(p.Dir, names[0]))
	b, _ := os.ReadFile(filepath.Join(p.Dir, names[1]))
	if string(a) != "orig" || string(b) != "one" {
		t.Fatalf("backups = %q, %q", a, b)
	}
}

func TestEditFileWithBackupWritesThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(real, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	err := EditFileWithBackup(link, editPolicy(dir), time.Unix(1_800_000_000, 0), func([]byte) ([]byte, error) {
		return []byte("b"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v %v", info, err)
	}
	if got, _ := os.ReadFile(real); string(got) != "b" {
		t.Fatalf("target = %q", got)
	}
}

func TestEditFileWithBackupCheckedRejectsChangeBeforeReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	changed := make(chan error, 1)
	go func() {
		<-ready
		changed <- os.WriteFile(path, []byte("external"), 0o600)
	}()
	err := EditFileWithBackupChecked(path, editPolicy(dir), time.Unix(1_800_000_000, 0),
		func([]byte) ([]byte, error) { return []byte("edited"), nil },
		func() error {
			close(ready)
			if err := <-changed; err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if string(data) != "original" {
				return errors.New("destination changed")
			}
			return nil
		})
	if err == nil {
		t.Fatal("concurrent change was accepted")
	}
	if got, _ := os.ReadFile(path); string(got) != "external" {
		t.Fatalf("destination = %q, want external change preserved", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}
