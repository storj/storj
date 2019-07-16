// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// Satellites queries satellite related info from db
type Satellites interface {
	// GetIDs returns list of satelliteIDs that storagenode has interacted with
	// at least once
	GetIDs(ctx context.Context, from, to time.Time) (storj.NodeIDList, error)
}
