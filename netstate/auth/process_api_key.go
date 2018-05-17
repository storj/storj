// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/subtle"

	"github.com/spf13/viper"
)

// ValidateAPIKey : validates the X-API-Key header to an env/flag input
func ValidateAPIKey(header string) bool {

	var apiKeyByte = []byte(viper.GetString("key"))
	var xAPIKeyByte = []byte(header)

	switch {
	case len(apiKeyByte) == 0:
		return false
	case len(apiKeyByte) > 0:
		result := subtle.ConstantTimeCompare(apiKeyByte, xAPIKeyByte)
		if result == 1 {
			return true
		}
	}
	return false
}
