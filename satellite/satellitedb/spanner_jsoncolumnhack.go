// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strconv"
)

// hackyResolveSpannerJSONColumn normalizes an sql.NullString from a dbx-generated JSON column query.
// This handles an issue where dbx base64-encodes JSON data when writing to Spanner.
func hackyResolveSpannerJSONColumn(ns sql.NullString) ([]byte, bool) {
	if !ns.Valid {
		return nil, false
	}

	txt := ns.String
	if txt == "" || txt == "null" {
		return nil, false
	}

	txt = unquoteIfQuoted(txt)

	if dec, err := base64.StdEncoding.DecodeString(txt); err == nil && json.Valid(dec) {
		return dec, true
	}

	return []byte(txt), true
}

func unquoteIfQuoted(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}
