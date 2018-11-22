// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import "regexp"

// IsValid is a method for validating UserInfo entity
func (userInfo *UserInfo) IsValid() bool {
	regexpEmail := "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"
	re := regexp.MustCompile(regexpEmail)

	if userInfo.FirstName == "" {
		return false
	}
	if userInfo.LastName == "" {
		return false
	}
	if userInfo.Email == "" || !re.MatchString(userInfo.Email) {
		return false
	}
	if len(userInfo.Password) < 6 {
		return false
	}

	return true
}
