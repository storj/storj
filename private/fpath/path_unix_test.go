// Copyright (C) 2019 Storj Labs, Inc.
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
		{
			url:  "/home/user/folder",
			base: "folder",
		},
		{
			url:  "/home/user/folder/",
			base: "folder",
		},
		{
			url:  "/home/user/folder/file.sh",
			base: "file.sh",
		},
		{
			url:  "//home//user//folder//file.sh",
			base: "file.sh",
		},
	} {
		testLocalPath(t, tt.url, tt.base, i)
	}
}
