// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"github.com/zeebo/errs"
)

const (
	// PasswordMinimumLength is the minimum allowed length for user account passwords.
	PasswordMinimumLength = 6
	// PasswordMaximumLength is the maximum allowed length for user account passwords.
	PasswordMaximumLength = 128
)

// ErrValidation validation related error class.
var ErrValidation = errs.Class("validation")

// ValidatePassword validates password.
// It returns an plain error (not wrapped in a errs.Class) if pass is invalid.
func ValidatePassword(pass string) error {
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
