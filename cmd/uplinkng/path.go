// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strings"

	"github.com/zeebo/errs"
)

func parsePath(path string) (bucket, key string, ok bool, err error) {
	// Paths, Chapter 2, Verses 9 to 21.
	//
	// And the Devs spake, saying,
	// First shalt thou find the Special Prefix "sj:".
	// Then, shalt thou count two slashes, no more, no less.
	// Two shall be the number thou shalt count,
	// and the number of the counting shall be two.
	// Three shalt thou not count, nor either count thou one,
	// excepting that thou then proceed to two.
	// Four is right out!
	// Once the number two, being the second number, be reached,
	// then interpret thou thy Path as a remote path,
	// which being made of a bucket and key, shall split it.

	if strings.HasPrefix(path, "sj://") || strings.HasPrefix(path, "s3://") {
		unschemed := path[5:]
		bucketIdx := strings.IndexByte(unschemed, '/')

		// handles sj:// or sj:///foo
		if len(unschemed) == 0 || bucketIdx == 0 {
			return "", "", false, errs.New("invalid path: empty bucket in path: %q", path)
		}

		// handles sj://foo
		if bucketIdx == -1 {
			return unschemed, "", true, nil
		}

		return unschemed[:bucketIdx], unschemed[bucketIdx+1:], true, nil
	}
	return "", "", false, nil
}
