// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"github.com/zeebo/errs"
)

const (
	// PasswordMinimumLength is the minimum allowed length for user account passwords, based on NIST guidelines for user-generated passwords.
	PasswordMinimumLength = 8
	// PasswordMaximumLength is the maximum allowed length for user account passwords, based on NIST guidelines for user-generated passwords.
	PasswordMaximumLength = 64
)

// ErrValidation validation related error class.
var ErrValidation = errs.Class("validation")

// ValidateNewPassword validates password for creation.
// It returns an plain error (not wrapped in a errs.Class) if pass is invalid.
//
// Password minimum length has previously been as short as 6, and maximum as long as 128.
// Therefore, this validation should only be applied to new passwords - old passwords may have previously been created outside of the 8-64 length range.
func ValidateNewPassword(pass string) error {
	if len(pass) < PasswordMinimumLength {
		return errs.New(passwordTooShortErrMsg, PasswordMinimumLength)
	}

	if len(pass) > PasswordMaximumLength {
		return errs.New(passwordTooLongErrMsg, PasswordMaximumLength)
	}

	return nil
}

// ValidateFullName validates full name.
// It returns an plain error (not wrapped in a errs.Class) if name is invalid.
func ValidateFullName(name string) error {
	if name == "" {
		return errs.New("full name can not be empty")
	}

	return nil
}
