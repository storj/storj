// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

func bucketPath(bucket string) paths.Path {
	return paths.New(bucket)
}

func objectPath(bucket string, path storj.Path) paths.Path {
	return paths.New(path).Prepend(bucket)
}

// TODO: this is incorrect since there's no good way to get such a path
func firstToStartAfterPath(first string) paths.Path {
	startAfter := paths.New(first)
	if len(startAfter) > 0 {
		startAfter[len(startAfter)-1] = firstToStartAfter(startAfter[len(startAfter)-1])
	}
	return startAfter
}

func firstToStartAfter(first string) string {
	if first == "" {
		return ""
	}

	before := []byte(first)
	if before[len(before)-1] == 0 {
		return string(before[:len(before)-1])
	}
	before[len(before)-1]--

	before = append(before, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	return string(before)
}
