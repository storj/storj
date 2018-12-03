// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/pb"
)

var (
	ctx = context.Background()
)

func TestBandwidthAgreements(t *testing.T) {
	TS := newTestServer(t)
	defer TS.stop()

	pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, TS.K)
	assert.NoError(t, err)

	rba, err := GenerateRenterBandwidthAllocation(pba, TS.K)
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	_, err = TS.C.BandwidthAgreements(ctx, rba)
	assert.NoError(t, err)
}
