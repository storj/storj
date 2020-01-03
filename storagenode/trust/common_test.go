// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/trust"
)

func makeSatelliteURL(host string) trust.SatelliteURL {
	return trust.SatelliteURL{
		ID:   testrand.NodeID(),
		Host: host,
		Port: 7777,
	}
}
