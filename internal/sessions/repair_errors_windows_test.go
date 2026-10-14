//go:build windows

package sessions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRepairRefusesWindowsACLUnreadableSessionTree(t *testing.T) {
	for _, folder := range []string{"sessions", "archived_sessions/nested"} {
		t.Run(folder, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, folder)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			denyDirectoryListing(t, dir)
			withoutBackupPrivileges(t)
			if _, err := os.ReadDir(dir); !os.IsPermission(err) {
				t.Fatalf("ACL must deny enumeration: %v", err)
			}
			assertRepairFailsWithoutWriting(t, home)
		})
	}
}

// Hosted runners can have backup privileges enabled, which bypass a DACL
// when Go opens a directory with FILE_FLAG_BACKUP_SEMANTICS. Disable privileges
// only on this test thread so it exercises ordinary user access checks.
func withoutBackupPrivileges(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if err := windows.ImpersonateSelf(windows.SecurityImpersonation); err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.RevertToSelf(); err != nil {
			t.Fatal(err)
		}
		runtime.UnlockOSThread()
	})
	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, false, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	if err := windows.AdjustTokenPrivileges(token, true, nil, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func denyDirectoryListing(t *testing.T, dir string) {
	t.Helper()
	saved, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		acl, _, err := saved.DACL()
		if err == nil {
			err = windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
		}
		if err != nil {
			t.Error(err)
		}
	})
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	sd, err := windows.SecurityDescriptorFromString("D:P(D;;0x1;;;" + sid + ")(A;;FA;;;" + sid + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}
