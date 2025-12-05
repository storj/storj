// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHackySpannerJSONColumnResolver(t *testing.T) {
	type tc struct {
		name       string
		in         sql.NullString
		wantOK     bool
		wantEq     string // expected exact string after normalization (when wantOK).
		wantJSONOK bool   // whether output should be valid JSON.
	}

	// {"foo":"bar"} base64.
	const base64Obj = "eyJmb28iOiJiYXIifQ=="

	tests := []tc{
		{
			name:   "invalid NullString => nil,false",
			in:     sql.NullString{Valid: false},
			wantOK: false,
		},
		{
			name:   "empty string => nil,false",
			in:     sql.NullString{String: "", Valid: true},
			wantOK: false,
		},
		{
			name:   `"null" literal => nil,false`,
			in:     sql.NullString{String: "null", Valid: true},
			wantOK: false,
		},
		{
			name:       "plain JSON object passes through",
			in:         sql.NullString{String: `{"a":1}`, Valid: true},
			wantOK:     true,
			wantEq:     `{"a":1}`,
			wantJSONOK: true,
		},
		{
			name:       "quoted base64 of JSON decodes",
			in:         sql.NullString{String: `"` + base64Obj + `"`, Valid: true},
			wantOK:     true,
			wantEq:     `{"foo":"bar"}`,
			wantJSONOK: true,
		},
		{
			name:       "unquoted base64 of JSON decodes",
			in:         sql.NullString{String: base64Obj, Valid: true},
			wantOK:     true,
			wantEq:     `{"foo":"bar"}`,
			wantJSONOK: true,
		},
		{
			name:       "quoted JSON string (not base64) gets unquoted",
			in:         sql.NullString{String: `"plain string"`, Valid: true},
			wantOK:     true,
			wantEq:     `plain string`,
			wantJSONOK: false, // not a JSON object/array, just text.
		},
		{
			name:       "base64 decodes to non-JSON => keep original",
			in:         sql.NullString{String: "bm90anNvbg==", Valid: true}, // "notjson"
			wantOK:     true,
			wantEq:     "bm90anNvbg==",
			wantJSONOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := hackyResolveSpannerJSONColumn(tt.in)
			require.Equal(t, tt.wantOK, ok)
			if !ok {
				require.Nil(t, got)
				return
			}
			require.Equal(t, tt.wantEq, string(got))

			if tt.wantJSONOK {
				require.True(t, json.Valid(got), "expected valid JSON")
			} else {
				// If we don't expect valid JSON, make sure we don't accidentally claim it is.
				require.False(t, json.Valid(got), "did not expect valid JSON")
			}
		})
	}
}
