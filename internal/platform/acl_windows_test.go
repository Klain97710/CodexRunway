//go:build windows

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Read back the OS descriptor, including its ACE SID, rather than trusting
// chmod or the production setter's return value.
func assertWindowsOwnerOnly(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		t.Fatalf("read security descriptor: %v", err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL inheritance must be disabled: %v", err)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount != 1 {
		t.Fatalf("expected exactly one DACL ACE: %v", err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(acl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERITED_ACE != 0 || ace.Mask != 0x1f01ff {
		t.Fatal("ACE must explicitly allow file full control")
	}
	if !(*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(user.User.Sid) {
		t.Fatal("ACE must identify the current user")
	}
}

func allowEveryone(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsPrivateDirectoryReplacesPermissiveACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "accounts")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	allowEveryone(t, dir)
	if err := EnsurePrivateDirectory(dir); err != nil {
		t.Fatal(err)
	}
	assertWindowsOwnerOnly(t, dir)
}

func TestWindowsNewDirectoriesArePrivateBeforePostCreationProtection(t *testing.T) {
	root := t.TempDir()
	allowEveryone(t, root)
	dir := filepath.Join(root, "runtime", "sqlite", "version")
	if err := mkdirPrivate(dir); err != nil {
		t.Fatal(err)
	}
	for path := dir; path != root; path = filepath.Dir(path) {
		assertWindowsOwnerOnly(t, path)
	}
	if err := verifyOwnerOnly(root, 0o700); err == nil {
		t.Fatal("creation must not change an existing ancestor's ACL")
	}
}

func TestWindowsTemporaryCredentialIsPrivateBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	allowEveryone(t, dir)
	f, err := createAtomicTemp(dir, ".auth.*.tmp", true)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	assertWindowsOwnerOnly(t, f.Name())
	if info, err := f.Stat(); err != nil || info.Size() != 0 {
		t.Fatalf("permission check must precede credential content: %v", err)
	}
}

func windowsFileIdentity(t *testing.T, path string) [3]uint32 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		t.Fatal(err)
	}
	return [3]uint32{info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow}
}

func TestWindowsAtomicReplacementKeepsProtectedSourceACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "auth.json")
	if err := WriteFileAtomic(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertWindowsOwnerOnly(t, filepath.Dir(path))
	allowEveryone(t, path)
	before := windowsFileIdentity(t, path)
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte("new")) {
		t.Fatalf("replacement data differs: %v", err)
	}
	if before == windowsFileIdentity(t, path) {
		t.Fatal("atomic replacement must install the temporary file, not modify the old file in place")
	}
	assertWindowsOwnerOnly(t, path)
}

func TestWindowsLockFileAndDirectoryArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runway", "auth.json.lock")
	lock, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	assertWindowsOwnerOnly(t, filepath.Dir(path))
	assertWindowsOwnerOnly(t, path)
}

func TestWindowsDeniedAtomicReplacementPreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := WriteFileAtomic(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	// Refusing FILE_SHARE_DELETE forces MoveFileEx to fail rather than falling
	// back to truncating the original credential file.
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	if err := WriteFileAtomic(path, []byte("replacement"), 0o600); err == nil {
		t.Fatal("expected replacement to fail while deletion sharing is denied")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, []byte("original")) {
		t.Fatalf("original was changed on failure: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary credential was not removed: %v", err)
	}
}
