// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"regexp"
	"strings"
)

// IsValid is a method for validating UserInfo entity
func (userInfo *UserInfo) IsValid() bool {
	if userInfo.FirstName == "" {
		return false
	}
	if userInfo.LastName == "" {
		return false
	}
	if !validateEmail(userInfo.Email) {
		return false
	}
	if len(userInfo.Password) < 6 {
		return false
	}

	return true
}

// validateEmail is a function for email validation
func validateEmail(email string) bool {
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
