// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package logging

import (
	"fmt"
	"net/url"
)

// Redacted removes the password from a URL string.
func Redacted(source string) string {
	parsed, err := url.Parse(source)
	if err != nil {
		return fmt.Sprintf("redacting password from URL was not possible: %s", err.Error())
	}
	return parsed.Redacted()
}
