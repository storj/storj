// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"regexp"
	"strings"
)

// ErrMissingField is used to indicate that field of entity is missing
type ErrMissingField string

// ErrInvalidField is used to indicate that field of entity has invalid value
type ErrInvalidField string

// Error is a method for generating error an error for missing field
func (e ErrMissingField) Error() string {
	return string(e) + " is required"
}

// Error is a method for generating error an error for invalid field value
func (e ErrInvalidField) Error() string {
	return string(e) + " is invalid"
}

// IsValid is a method for validating UserInfo entity
func (userInfo *UserInfo) IsValid() error {
	if userInfo.FirstName == "" {
		return ErrMissingField("first name")
	}
	if userInfo.Email == "" {
		return ErrMissingField("email")
	}
	if !checkEmailAddressSyntax(userInfo.Email) {
		return ErrInvalidField("email")
	}
	if userInfo.Password == "" {
		return ErrMissingField("password")
	}
	if len(userInfo.Password) < 6 {
		return ErrInvalidField("password")
	}

	return nil
}

// checkEmailAddressSyntax is a function for email validation
func checkEmailAddressSyntax(email string) bool {
	localPartRegexp := regexp.MustCompile("^[a-zA-Z0-9!#$%&'*+/=?^_`{|}~.-]+$")
	noDotsRegexp := regexp.MustCompile("(^[.]{1})|([.]{1}$)|([.]{2,})")
	hostRegexp := regexp.MustCompile(string("^[^\\s]+\\.[^\\s]+$"))

	length := len(email)

	if length > 254 {
		return false
	}

	delimiter := strings.LastIndex(email, "@")
	if delimiter <= 0 || delimiter > len(email)-3 {
		return false
	}

	localPart := email[:delimiter]
	lenValid := len(localPart) <= 64
	validString := localPartRegexp.MatchString(localPart)
	noDots := !noDotsRegexp.MatchString(localPart)

	if !(lenValid && validString && noDots) {
		return false
	}

	hostPart := email[delimiter+1:]
	return hostRegexp.MatchString(hostPart)
}
