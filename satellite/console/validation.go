// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"strings"
	"unicode"

	"github.com/zeebo/errs"
)

const (
	passMinLength      = 6
	passMinNumberCount = 1
	passMinAZCount     = 1
)

// ErrValidation validation related error class
var ErrValidation = errs.Class("validation error")

// validationError is slice of ErrValidation class errors
type validationErrors []error

// Add new ErrValidation err
func (validation *validationErrors) Add(format string, args ...interface{}) {
	*validation = append(*validation, ErrValidation.New(format, args...))
}

// AddWrap adds new ErrValidation wrapped err
func (validation *validationErrors) AddWrap(err error) {
	*validation = append(*validation, ErrValidation.Wrap(err))
}

// Combine returns combined validation errors
func (validation *validationErrors) Combine() error {
	return errs.Combine(*validation...)
}

// countNumerics returns total number of digits in string
func countNumerics(s string) int {
	total := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			total++
		}
	}

	return total
}

// countLetters returns total number of letters in string
func countLetters(s string) int {
	total := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			total++
		}
	}

	return total
}

// validatePassword validates password
func validatePassword(pass string) error {
	var errs validationErrors

	if len(pass) < passMinLength {
		errs.Add("password can't be less than %d characters", passMinLength)
	}

	if countNumerics(pass) < passMinNumberCount {
		errs.Add("password should contain at least %d digits", passMinNumberCount)
	}

	if countLetters(pass) < passMinAZCount {
		errs.Add("password should contain at least %d alphabetic characters", passMinAZCount)
	}

	return errs.Combine()
}

// normalizeEmail converts emails with different casing into equal strings
// Note: won't work with µıſͅςϐϑϕϖϰϱϵᲀᲁᲂᲃᲄᲅᲆᲇᲈẛι
func normalizeEmail(s string) string {
	return strings.ToLower(s)
}
