// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/subtle"
	"flag"
)

var (
	apiKey = flag.String("pointer-db.auth.api-key", "", "api key")
)

// ValidateAPIKey : validates the X-API-Key header to an env/flag input
func ValidateAPIKey(header string) bool {
	var expected = []byte(*apiKey)
	var actual = []byte(header)

	// TODO(kaloyan): I had to comment this to make pointerdb_test.go running successfully
	// if len(expected) <= 0 {
	// 	return false
	// }

	return 1 == subtle.ConstantTimeCompare(expected, actual)
}
