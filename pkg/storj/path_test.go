// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitPath(t *testing.T) {
	for i, tt := range []struct {
		path  string
		comps []string
	}{
		{"", []string{""}},
		{"/", []string{"", ""}},
		{"//", []string{"", "", ""}},
		{" ", []string{" "}},
		{"a", []string{"a"}},
		{"/a/", []string{"", "a", ""}},
		{"a/b/c/d", []string{"a", "b", "c", "d"}},
		{"///a//b////c/d///", []string{"", "", "", "a", "", "b", "", "", "", "c", "d", "", "", ""}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.comps, SplitPath(tt.path), errTag)
	}
}

func TestJoinPaths(t *testing.T) {
	for i, tt := range []struct {
		comps []string
		path  string
	}{
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, "/"},
		{[]string{"/", ""}, "//"},
		{[]string{"/", "/"}, "///"},
		{[]string{"", "", ""}, "//"},
		{[]string{" "}, " "},
		{[]string{"a"}, "a"},
		{[]string{"", "a", ""}, "/a/"},
		{[]string{"a", "b", "c", "d"}, "a/b/c/d"},
		{[]string{"a/b", "c/d"}, "a/b/c/d"},
		{[]string{"a/b/", "c/d"}, "a/b//c/d"},
		{[]string{"", "", "", "a", "", "b", "", "", "", "c", "d", "", "", ""}, "///a//b////c/d///"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		assert.Equal(t, tt.path, JoinPaths(tt.comps...), errTag)
	}
}
