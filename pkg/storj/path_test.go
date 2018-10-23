// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathComponents(t *testing.T) {
	for i, tt := range []struct {
		path  string
		comps []string
	}{
		{"", []string{}},
		{"/", []string{}},
		{"//", []string{}},
		{" ", []string{" "}},
		{"a", []string{"a"}},
		{"/a/", []string{"a"}},
		{"a/b/c/d", []string{"a", "b", "c", "d"}},
		{"///a//b////c/d///", []string{"a", "b", "c", "d"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.comps, PathComponents(tt.path), errTag)
	}
}

func TestTrimLeftPathComponents(t *testing.T) {
	for i, tt := range []struct {
		path    string
		num     int
		trimmed string
	}{
		{"", 0, ""},
		{"", 1, ""},
		{" ", 0, " "},
		{" ", 1, ""},
		{"a", 0, "a"},
		{"a", 1, ""},
		{"a/b/c/d", 0, "a/b/c/d"},
		{"a/b/c/d", 2, "c/d"},
		{"a/b/c/d", 5, ""},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.trimmed, TrimLeftPathComponents(tt.path, tt.num), errTag)
	}
}
