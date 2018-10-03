// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"sync"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/meta"
)

// Audit to audit segments
type Audit struct {
	pointers pdbclient.Client
	r        *rand.Rand
	lastPath *paths.Path
	mutex    sync.Mutex
}

// NewAudit creates a new instance of audit
func NewAudit(pointers pdbclient.Client) *Audit {
	return &Audit{
		pointers: pointers,
	}
}

// Stripe is a struct that contains stripe info
type Stripe struct {
	stripe int
}

// NextStripe returns a random stripe to be audited
func (a *Audit) NextStripe(ctx context.Context) (stripe *Stripe, err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// retreive a random list of pointers
	var pointerItems []pdbclient.ListItem

	// need to get random limit
	if a.lastPath == nil {
		pointerItems, _, err = a.pointers.List(ctx, nil, nil, nil, true, 10, meta.None)
	} else {
		pointerItems, _, err = a.pointers.List(ctx, nil, *a.lastPath, nil, true, 10, meta.None)
	}

	if err != nil {
		return nil, err
	}

	if len(pointerItems) == 0 {
		return nil, ErrNoPointers
	}

	randomNum := rand.Intn(len(pointerItems))
	pointerItem := pointerItems[randomNum]

	// get a pointer
	path := pointerItem.Path
	pointer, err := a.pointers.Get(ctx, path)

	if err != nil {
		return nil, err
	}

	// keep track of last path used
	if a.lastPath != &path {
		a.lastPath = &path
	} else {
		// get another path
		pointerItem := pointerItems[randomNum]
		path := pointerItem.Path
		a.lastPath = &path
	}

	// create the erasure scheme so we can get the stripe size
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, err
	}

	// get random stripe
	stripeSize := es.StripeSize()
	stripeNum := rand.Intn((int(pointer.GetSize()) / stripeSize))
	if stripeNum == 0 {
		stripeNum = stripeNum + 1
	}

	return &Stripe{
		stripeNum,
	}, nil
}

// create the erasure scheme
func makeErasureScheme(rs *pb.RedundancyScheme) (eestream.ErasureScheme, error) {
	fc, err := infectious.NewFEC(int(rs.GetMinReq()), int(rs.GetTotal()))
	if err != nil {
		return nil, err
	}
	es := eestream.NewRSScheme(fc, int(rs.GetErasureShareSize()))
	return es, nil
}
