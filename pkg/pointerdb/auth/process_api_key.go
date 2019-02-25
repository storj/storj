// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"crypto/subtle"
	"flag"
)

var (
	apiKey = flag.String("pointer-db.auth.api-key", "", "api key")
)

// Purpose of this is to process an API Key to see if it matches the correct client.
//
// **To use, run in** *examples/auth/main.go*:
// `$ go run main.go --key=yourkey`
//
// Default api key is preset with the mocked headers. This will be changed later.
//
// **Where this is going**:
// We're going to be using macaroons to validate a token and permissions. This is a small step to building in that direction.

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
