// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"fmt"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorjURL(t *testing.T) {
	for i, tt := range []struct {
		url    string
		bucket string
		path   string
		base   string
		joint  string
	}{
		{
			url:    "sj:/mybucket",
			bucket: "mybucket",
			path:   "",
			base:   "",
			joint:  "suffix",
		},
		{
			url:    "sj:/mybucket/",
			bucket: "mybucket",
			path:   "",
			base:   "",
			joint:  "suffix",
		},
		{
			url:    "sj:/mybucket/myfile",
			bucket: "mybucket",
			path:   "myfile",
			base:   "myfile",
			joint:  "suffix",
		},
		{
			url:    "sj://mybucket",
			bucket: "mybucket",
			path:   "",
			base:   "",
			joint:  "suffix",
		},
		{
			url:    "sj://mybucket/",
			bucket: "mybucket",
			path:   "",
			base:   "",
			joint:  "suffix",
		},
		{
			url:    "sj://mybucket/myfile",
			bucket: "mybucket",
			path:   "myfile",
			base:   "myfile",
			joint:  "myfile/suffix",
		},
		{
			url:    "sj://mybucket/myfile/",
			bucket: "mybucket",
			path:   "myfile",
			base:   "myfile",
			joint:  "myfile/suffix",
		},
		{
			url:    "sj://mybucket/myfolder/myfile",
			bucket: "mybucket",
			path:   "myfolder/myfile",
			base:   "myfile",
			joint:  "myfolder/myfile/suffix",
		},
		{
			url:    "sj://mybucket///myfolder///myfile",
			bucket: "mybucket",
			path:   "myfolder/myfile",
			base:   "myfile",
			joint:  "myfolder/myfile/suffix",
		},
		{
			url:    "sj:////mybucket///myfolder///myfile",
			bucket: "mybucket",
			path:   "myfolder/myfile",
			base:   "myfile",
			joint:  "myfolder/myfile/suffix",
		},
		{
			url:    "s3:////mybucket///myfolder///myfile",
			bucket: "mybucket",
			path:   "myfolder/myfile",
			base:   "myfile",
			joint:  "myfolder/myfile/suffix",
		},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		fp, err := New(tt.url)
		assert.NoError(t, err, errTag)

		assert.Equal(t, false, fp.IsLocal(), errTag)
		assert.Equal(t, tt.bucket, fp.Bucket(), errTag)
		assert.Equal(t, tt.path, fp.Path(), errTag)
		assert.Equal(t, tt.base, fp.Base(), errTag)
		assert.Equal(t, path.Join(tt.path, "suffix"), fp.Join("suffix").Path(), errTag)
		assert.Equal(t, tt.url+"/suffix", fp.Join("suffix").String(), errTag)
		assert.Equal(t, tt.url, fp.String(), errTag)
	}
}

func TestInvalidStorjURL(t *testing.T) {
	for i, tt := range []string{
		"://",
		"sj:bucket",
		"sj://",
		"sj:///",
		"sj://mybucket:8080/",
		"sj:///mybucket:8080/",
		"sj:////mybucket:8080/",
		"http://bucket/file.txt",
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		_, err := New(tt)
		assert.Error(t, err, errTag)
	}
}

func TestLocalPath(t *testing.T) {
	for i, tt := range []struct {
		url  string
		base string
	}{
		{
			url:  "-",
			base: "-",
		},
		{
			url:  "",
			base: ".",
		},
		{
			url:  ".",
			base: ".",
		},
		{
			url:  "..",
			base: "..",
		},
		{
			url:  "/a/b/c",
			base: "c",
		},
		{
			url:  "a",
			base: "a",
		},
		{
			url:  "a/b/c",
			base: "c",
		},
		{
			url:  "///a/b/c",
			base: "c",
		},
	} {
		testLocalPath(t, tt.url, tt.base, i)
	}
}

func testLocalPath(t *testing.T, url, base string, i int) {
	errTag := fmt.Sprintf("Test case #%d", i)

	fp, err := New(url)
	assert.NoError(t, err, errTag)

	assert.Equal(t, true, fp.IsLocal(), errTag)
	assert.Equal(t, "", fp.Bucket(), errTag)
	assert.Equal(t, url, fp.Path(), errTag)
	assert.Equal(t, base, fp.Base(), errTag)
	assert.Equal(t, filepath.Join(url, "suffix"), fp.Join("suffix").Path(), errTag)
	assert.Equal(t, filepath.Join(url, "suffix"), fp.Join("suffix").String(), errTag)
	assert.Equal(t, url, fp.String(), errTag)
}
