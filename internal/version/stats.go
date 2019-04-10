// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import "hash/crc32"

func (v *Info) Stats(reportValue func(name string, val float64)) {
	if v.Release {
		reportValue("release", 1)
	} else {
		reportValue("release", 0)
	}
	reportValue("timestamp", float64(v.Timestamp.Unix()))

	v.crcOnce.Do(func() {
		c := crc32.NewIEEE()
		_, err := c.Write([]byte(buildCommitHash))
		if err != nil {
			panic(err)
		}
		v.commitHashCRC = c.Sum32()
	})

	reportValue("commit", float64(v.commitHashCRC))
	reportValue("major", float64(v.Version.Major))
	reportValue("minor", float64(v.Version.Minor))
	reportValue("patch", float64(v.Version.Patch))
}
