// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build linux darwin

package fpath

import (
	"testing"
)

func TestLocalPathUnix(t *testing.T) {
	for i, tt := range []struct {
		url  string
		base string
	}{
		{
			url:  "/",
			base: "/",
		},
		{
			url:  "//",
			base: "/",
		},
	} {
		testLocalPath(t, tt.url, tt.base, i)
	}
}
