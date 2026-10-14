//go:build windows

package platform

import (
	"errors"
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ownerSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	// P disables inheritance; OI/CI keep newly created children private until
	// they receive their own protected DACL. There is exactly one allowed SID.
	return windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
}

func restrictToOwner(path string) error {
	sd, err := ownerSecurityDescriptor()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		return err
	}
	return verifyOwnerOnly(path, 0)
}

func verifyOwnerOnly(path string, _ fs.FileMode) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	if sd == nil {
		return errors.New("platform: missing Windows security descriptor")
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 || acl == nil || acl.AceCount != 1 {
		return errors.New("platform: Windows DACL is not protected and owner-only")
	}
	return verifyOwnerACE(acl)
}

func verifyOwnerACE(acl *windows.ACL) error {
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(acl, 0, &ace); err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	const fileAllAccess = 0x1f01ff // FILE_ALL_ACCESS, the SDDL FA mask for files.
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
		ace.Header.AceFlags&windows.INHERITED_ACE != 0 ||
		ace.Mask != fileAllAccess || !sid.Equals(user.User.Sid) {
		return errors.New("platform: Windows DACL does not grant only the current user full control")
	}
	return nil
}
