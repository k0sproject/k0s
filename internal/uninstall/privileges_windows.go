//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package uninstall

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

func ensurePrivileges() error {
	token, err := openImpersonationToken()
	if err != nil {
		return err
	}
	defer token.Close()

	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("failed to get administrators SID: %w", err)
	}

	isAdmin, err := token.IsMember(sid)
	if err != nil {
		return fmt.Errorf("failed to verify administrator privileges: %w", err)
	}
	if !isAdmin {
		return errors.New("this command must be run with administrator privileges")
	}
	return nil
}

func openImpersonationToken() (windows.Token, error) {
	var processToken windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &processToken); err != nil {
		return 0, fmt.Errorf("failed to open process token: %w", err)
	}
	defer processToken.Close()

	var impersonationToken windows.Token
	if err := windows.DuplicateTokenEx(processToken, windows.TOKEN_QUERY, nil, windows.SecurityIdentification, windows.TokenImpersonation, &impersonationToken); err != nil {
		return 0, fmt.Errorf("failed to duplicate process token: %w", err)
	}
	return impersonationToken, nil
}
