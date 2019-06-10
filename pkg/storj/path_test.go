// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathIterator(t *testing.T) {
	for i, tt := range []struct {
		path  string
		comps []string
	}{
		{"", []string{}},
		{"/", []string{"", ""}},
		{"//", []string{"", "", ""}},
		{" ", []string{" "}},
		{"a", []string{"a"}},
		{"/a/", []string{"", "a", ""}},
		{"a/b/c/d", []string{"a", "b", "c", "d"}},
		{"///a//b////c/d///", []string{"", "", "", "a", "", "b", "", "", "", "c", "d", "", "", ""}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		iter, got := PathIterator{raw: tt.path}, make([]string, 0, len(tt.comps))
		for !iter.Done() {
			got = append(got, iter.Next())
		}
		assert.Equal(t, tt.comps, got, errTag)
	}
}

