// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"hash/crc32"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"
)

// Stats implements the monkit.StatSource interface
func (info *Info) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	key := monkit.NewSeriesKey("version_info")

	if info.Release {
		cb(key, "release", 1)
	} else {
		cb(key, "release", 0)
	}
	if !info.Timestamp.IsZero() {
		cb(key, "timestamp", float64(info.Timestamp.Unix()))
	}
	crc := atomic.LoadUint32(&info.commitHashCRC)
	if crc == 0 {
		c := crc32.NewIEEE()
		_, err := c.Write([]byte(buildCommitHash))
		if err != nil {
			panic(err)
		}
		atomic.StoreUint32(&info.commitHashCRC, c.Sum32())
	}
	cb(key, "commit", float64(crc))
	cb(key, "major", float64(info.Version.Major))
	cb(key, "minor", float64(info.Version.Minor))
	cb(key, "patch", float64(info.Version.Patch))
}
