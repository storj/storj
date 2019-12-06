// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package useragent_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/useragent"
)

func TestUserAgent(t *testing.T) {
	type testcase struct {
		in   string
		info useragent.Info
	}

	var tests = []testcase{
		{"Hello", useragent.Info{useragent.Product{"Hello", ""}, "Hello"}},
		{"Hello/1.0", useragent.Info{useragent.Product{"Hello", "1.0"}, "Hello/1.0"}},
		{"Hello/1.0+version#123", useragent.Info{useragent.Product{"Hello", "1.0+version#123"}, "Hello/1.0+version#123"}},
	}

	for _, test := range tests {
		info, err := useragent.Parse(test.in)
		require.NoError(t, err)
		require.Equal(t, test.info, info)
	}
}
