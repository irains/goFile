//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func protectPrivateDirectory(path string) error {
	return protectPrivatePath(path, windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE)
}

func protectPrivateFile(path string) error {
	return protectPrivatePath(path, windows.NO_INHERITANCE)
}

func protectPrivatePath(path string, inheritance uint32) error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return errors.New("could not inspect the runtime-state owner")
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return errors.New("could not inspect the runtime-state owner")
	}
	owner, err := user.User.Sid.Copy()
	if err != nil {
		return errors.New("could not inspect the runtime-state owner")
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(owner),
		},
	}}, nil)
	if err != nil {
		return errors.New("could not protect runtime state")
	}
	securityInfo := windows.SECURITY_INFORMATION(
		windows.OWNER_SECURITY_INFORMATION |
			windows.DACL_SECURITY_INFORMATION |
			windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, owner, nil, acl, nil); err != nil {
		return fmt.Errorf("could not protect runtime state: %w", err)
	}
	return nil
}
