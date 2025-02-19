// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		expected   string
	}{
		{"plain", "", "``"}, // technically illegal; can't have an empty identifier
		{"just backslash", "\\", "`\\\\`"},
		{"just backtick", "`", "`\\``"},
		{"letter", "a", "`a`"},
		{"dotted identifier", "a.b", "`a.b`"},
		{"backtick in the middle", "a`b", "`a\\`b`"},
		{"backslash in the middle", "a\\b", "`a\\\\b`"},
		{"raw string", `a\b`, "`a" + `\\` + "b`"},
		{"backslash and backtick", "\\a\\`", "`\\\\a\\\\\\``"},
		{"more backslashes and backtick", "\\a\\\\`", "`\\\\a\\\\\\\\\\``"},
		{"double backtick", "a``b", "`a\\`\\`b`"},
		{"triple backtick", "```", "`\\`\\`\\``"},
		{"double backslash", `\\`, "`\\\\\\\\`"},
		{"contains nul", "a\x00b", "`a\x00b`"}, // spec says "can contain any characters"
		{"newline", "\n", "`\n`"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := QuoteIdentifier(test.identifier)
			assert.Equal(t, test.expected, actual)
		})
	}
}
