// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func prependExtension(path, ext string) string {
	originalExt := filepath.Ext(path)
	dir, base := filepath.Split(path)
	base = base[:len(base)-len(originalExt)]
	return filepath.Join(dir, base+"."+ext+originalExt)
}

func parseDownloadURL(template string) string {
	url := strings.Replace(template, "{os}", runtime.GOOS, 1)
	url = strings.Replace(url, "{arch}", runtime.GOARCH, 1)
	return url
}

func createPattern(url string) string {
	_, binary := path.Split(url)
	if ext := path.Ext(binary); ext != "" {
		return binary[:len(binary)-len(ext)] + ".*" + ext
	}

	return binary + ".*"
}
