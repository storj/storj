// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"hash/crc32"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3"
)

// Stats implements the monkit.StatSource interface
func (v *Info) Stats(cb func(series monkit.Series, val float64)) {
	if v.Release {
		cb(monkit.NewSeries("version_info", "release"), 1)
	} else {
		cb(monkit.NewSeries("version_info", "release"), 0)
	}
	cb(monkit.NewSeries("version_info", "timestamp"), float64(v.Timestamp.Unix()))
	crc := atomic.LoadUint32(&v.commitHashCRC)
	if crc == 0 {
		c := crc32.NewIEEE()
		_, err := c.Write([]byte(buildCommitHash))
		if err != nil {
			panic(err)
		}
		atomic.StoreUint32(&v.commitHashCRC, c.Sum32())
	}
	cb(monkit.NewSeries("version_info", "commit"), float64(crc))
	cb(monkit.NewSeries("version_info", "major"), float64(v.Version.Major))
	cb(monkit.NewSeries("version_info", "minor"), float64(v.Version.Minor))
	cb(monkit.NewSeries("version_info", "patch"), float64(v.Version.Patch))
}
