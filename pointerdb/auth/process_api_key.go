// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/subtle"
	"net/http"

	"github.com/spf13/viper"
)

// InitializeHeaders : mocks HTTP headers to preset X-API-Key
func InitializeHeaders() *http.Header {

	httpHeaders := http.Header{
		"Accept-Encoding": {"gzip, deflate"},
		"Accept-Language": {"en-US,en;q=0.9"},
		"X-Api-Key":       {"12345"},
		"Cache-Control":   {"max-age=0"},
		"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"},
		"Connection":      {"keep-alive"},
	}
	return &httpHeaders
}

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
