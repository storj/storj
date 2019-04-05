// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import "hash/crc32"

func (v *Info) Stats(cb func(name string, val float64)) {
	if v.Release {
		cb("release", 1)
	} else {
		cb("release", 0)
	}
	cb("timestamp", float64(v.Timestamp.Unix()))

	v.crcOnce.Do(func() {
		c := crc32.NewIEEE()
		_, err := c.Write([]byte(buildCommitHash))
		if err != nil {
			panic(err)
		}
		v.commitHashCRC = c.Sum32()
	})

	cb("commit", float64(v.commitHashCRC))
	cb("major", float64(v.Version.Major))
	cb("minor", float64(v.Version.Minor))
	cb("patch", float64(v.Version.Patch))
}
