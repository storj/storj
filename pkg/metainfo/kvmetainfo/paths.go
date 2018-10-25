// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

/*
func bucketPath(bucket string) paths.Path {
	return paths.New(bucket)
}

func objectPath(bucket string, path storj.Path) paths.Path {
	return paths.New(path).Prepend(bucket)
}

*/

// TODO: known issue:
//   this is incorrect since there's no good way to get such a path
//   since the exact previous key is
//     append(previousPrefix(cursor), infinite(0xFF)...)
func keyBefore(cursor string) string {
	if cursor == "" {
		return ""
	}

	before := []byte(cursor)
	if before[len(before)-1] == 0 {
		return string(before[:len(before)-1])
	}
	before[len(before)-1]--

	before = append(before, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	return string(before)
}

func keyAfter(cursor string) string {
	return cursor + "\x00"
}
