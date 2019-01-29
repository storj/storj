// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
)

var (
	mon = monkit.Package()
)

// Config contains everything necessary for a server
type Config struct {
	Path                    string        `help:"path to store data in" default:"$CONFDIR/storage"`
	WhitelistedSatelliteIDs string        `help:"a comma-separated list of approved satellite node ids" default:""`
	SatelliteIDRestriction  bool          `help:"if true, only allow data from approved satellites" default:"false"`
	AllocatedDiskSpace      memory.Size   `user:"true" help:"total allocated disk space in bytes" default:"1TB"`
	AllocatedBandwidth      memory.Size   `user:"true" help:"total allocated bandwidth in bytes" default:"500GiB"`
	KBucketRefreshInterval  time.Duration `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`

	AgreementSenderCheckInterval time.Duration `help:"duration between agreement checks" default:"1h0m0s"`
	CollectorInterval            time.Duration `help:"interval to check for expired pieces" default:"1h0m0s"`
}
