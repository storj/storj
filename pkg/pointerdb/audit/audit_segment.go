// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"math/big"
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
	Index int
}

// NextStripe returns a random stripe to be audited
func (audit *Audit) NextStripe(ctx context.Context) (stripe *Stripe, more bool, err error) {
	audit.mutex.Lock()
	defer audit.mutex.Unlock()

	var pointerItems []pdbclient.ListItem
	var path paths.Path

	if audit.lastPath == nil {
		pointerItems, more, err = audit.pointers.List(ctx, nil, nil, nil, true, 0, meta.None)
	} else {
		pointerItems, more, err = audit.pointers.List(ctx, nil, *audit.lastPath, nil, true, 0, meta.None)
	}

	if err != nil {
		return nil, more, err
	}

	// get random pointer
	pointerItem, err := getRandomPointer(pointerItems)
	if err != nil {
		return nil, more, err
	}

	// get path
	path = pointerItem.Path

	// keep track of last path listed
	if !more {
		audit.lastPath = nil
	} else {
		audit.lastPath = &pointerItems[len(pointerItems)-1].Path
	}

	// get pointer info
	pointer, err := audit.pointers.Get(ctx, path)
	if err != nil {
		return nil, more, err
	}

	// create the erasure scheme so we can get the stripe size
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, more, err
	}

	//get random stripe
	index, err := getRandomStripe(es, pointer)
	if err != nil {
		return nil, more, err
	}

	return &Stripe{Index: index}, more, nil
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

func getRandomStripe(es eestream.ErasureScheme, pointer *pb.Pointer) (index int, err error) {
	stripeSize := es.StripeSize()
	randomStripeIndex, err := rand.Int(rand.Reader, big.NewInt(pointer.GetSize()/int64(stripeSize)))
	if err != nil {
		return -1, err
	}
	return int(randomStripeIndex.Int64()), nil
}

func getRandomPointer(pointerItems []pdbclient.ListItem) (pointer pdbclient.ListItem, err error) {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(len(pointerItems))))
	if err != nil {
		return pdbclient.ListItem{}, err
	}
	randomNumInt64 := randomNum.Int64()
	pointerItem := pointerItems[randomNumInt64]
	return pointerItem, nil
}
