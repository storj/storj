// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"testing"
)

func TestLocalPathWindows(t *testing.T) {
	for i, tt := range []struct {
		url  string
		base string
	}{
		{
			url:  `\`,
			base: `\`,
		},
		{
			url:  `\\`,
			base: `\`,
		},
		{
			url:  `c:\`,
			base: `\`,
		},
		{
			url:  `c:\`,
			base: `\`,
		},
		{
			url:  `c:\a\b\c`,
			base: `c`,
		},
		{
			url:  `c:\\a\\b\\c`,
			base: `c`,
		},
		{
			url:  `c:/a/b/c`,
			base: `c`,
		},
		{
			url:  `c://a//b//c`,
			base: `c`,
		},
		{
			url:  `c:\a/b\c`,
			base: `c`,
		},
		{
			url:  `\\a/b\c`,
			base: `c`,
		},
		{
			url:  `a\b\c`,
			base: `c`,
		},
		{
			url:  `a/b/c`,
			base: `c`,
		},
		{
			url:  `\\\a\b\c`,
			base: `c`,
		},
		{
			url:  `///a/b/c`,
			base: `c`,
		},
		{
			url:  `\\\unc\a\b\c`,
			base: `c`,
		},
		{
			url:  `///unc/a/b/c`,
			base: `c`,
		},
		{
			url:  `\\?\UNC\a\b\c`,
			base: `c`,
		},
		{
			url:  `\\?\C:\a\b\c`,
			base: `c`,
		},
		{
			url:  `\\?\C:\\a\\b\\c`,
			base: `c`,
		},
		{
			url:  `C:\a\b\`,
			base: `b`,
		},
		{
			url:  `C:\a\b\c.txt:extended`,
			base: `c.txt:extended`,
		},
		{
			url:  `\\a\b\c.txt:extended`,
			base: `c.txt:extended`,
		},
	} {
		testLocalPath(t, tt.url, tt.base, i)
	}
}
