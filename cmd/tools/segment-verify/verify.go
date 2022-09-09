// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/errs"
)

// ErrNodeOffline is returned when it was not possible to contact a node or the node was not responding.
var ErrNodeOffline = errs.Class("node offline")

// VerifyBatch verifies a single batch.
func (service *Service) VerifyBatch(ctx context.Context, batch *Batch) error {
	return errs.New("todo")
}
