// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"time"

	"github.com/zeebo/errs"
)

// WaitForStorageNodeEndpoints waits for storage node endpoints to finish their work.
// The call will return an error if they have not been completed after 1 minute.
func (planet *Planet) WaitForStorageNodeEndpoints(ctx context.Context) error {
	timeout := time.NewTimer(time.Minute)
	defer timeout.Stop()
	for {
		if planet.storageNodeLiveRequestCount() == 0 {
			return nil
		}

		select {
		case <-time.After(50 * time.Millisecond):
		case <-timeout.C:
			return errs.New("timed out waiting for storagenode endpoints")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (planet *Planet) storageNodeLiveRequestCount() int {
	total := 0
	for _, storageNode := range planet.StorageNodes {
		total += int(storageNode.Storage2.Endpoint.TestLiveRequestCount())
	}
	return total
}

// WaitForStorageNodeDeleters calls the Wait method on each storagenode's PieceDeleter.
func (planet *Planet) WaitForStorageNodeDeleters(ctx context.Context) {
	for _, sn := range planet.StorageNodes {
		sn.Peer.Storage2.PieceDeleter.Wait(ctx)
	}
}
