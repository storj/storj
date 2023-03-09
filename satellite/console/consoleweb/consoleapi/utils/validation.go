// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import "regexp"

// ValidateEmail validates email to have correct form and syntax.
func ValidateEmail(email string) bool {
	// This regular expression was built according to RFC 5322 and then extended to include international characters.
	re := regexp.MustCompile(`^(?:[a-z0-9\p{L}!#$%&'*+/=?^_{|}~\x60-]+(?:\.[a-z0-9\p{L}!#$%&'*+/=?^_{|}~\x60-]+)*|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*")@(?:(?:[a-z0-9\p{L}](?:[a-z0-9\p{L}-]*[a-z0-9\p{L}])?\.)+[a-z0-9\p{L}](?:[a-z\p{L}]*[a-z\p{L}])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9\p{L}-]*[a-z0-9\p{L}]:(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21-\x5a\x53-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])+)\])$`)
	match := re.MatchString(email)

	return match
}
