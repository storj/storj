// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package logging

import (
	"net/url"
	"strings"
)

// Redacted removes the password from a URL string.
func Redacted(source string) string {
	parsed, err := url.Parse(source)
	if err != nil {
		// This intentionally does not include the error information,
		// because the parse error will contain the source verbatim.
		return "ERROR: redacting password from URL was not possible"
	}

	query := parsed.Query()
	for k := range query {
		k = strings.ToLower(k)
		if strings.Contains(k, "pass") || strings.Contains(k, "credential") {
			delete(query, k)
		}
	}
	parsed.RawQuery = query.Encode()

	return parsed.Redacted()
}
