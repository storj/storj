// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateURL takes in a url and returns a boolean if it's valid
func ValidateURL(dst string) bool {
	parsed, err := url.Parse(dst)
	if err != nil {
		fmt.Printf("error parsing URL: %+v", err)
		return false
	}

	if parsed.Scheme == "" {
		fmt.Printf("url must have a valid scheme %s", parsed.Scheme)
		return false
	}

	if strings.Contains(dst, "///") {
		fmt.Printf("Invalid formatting in URL")
		return false
	}

	return strings.Contains(dst, "://")
}
