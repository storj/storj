// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	//"math/rand"

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
	//r        *rand.Rand
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

	fmt.Println("start after at fn call: ", a.lastPath)

	// retreive a random list of pointers
	var pointerItems []pdbclient.ListItem
	var path paths.Path

	// need to get random limit
	if a.lastPath == nil {
		pointerItems, _, err = a.pointers.List(ctx, nil, nil, nil, true, 10, meta.None)
	} else {
		pointerItems, _, err = a.pointers.List(ctx, nil, *a.lastPath, nil, true, 10, meta.None)
	}

	fmt.Println("pointerItems, ", pointerItems)
	fmt.Println("length of pointeritems; ", len(pointerItems))
	if err != nil {
		return nil, err
	}

	if len(pointerItems) == 0 {
		a.lastPath = nil
		return nil, ErrNoPointers
	}

	//pointerLength := len(pointerItems)
	randomInt, err := generateRandomNumber(len(pointerItems))
	if err != nil {
		return nil, err
	}

	pointerItem := pointerItems[randomInt]

	// get path
	path = pointerItem.Path

	// keep track of last path used
	if a.lastPath != &path {
		a.lastPath = &path
	} else {
		return nil, ErrNoPointers
	}

	// get pointer info
	pointer, err := a.pointers.Get(ctx, path)
	if err != nil {
		return nil, ErrNoPointers
	}

	// create the erasure scheme so we can get the stripe size
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, err
	}

	//get random stripe
	stripeSize := es.StripeSize()
	randomStripeNum, err := rand.Int(rand.Reader, big.NewInt(int64(pointer.GetSize())/int64(stripeSize)))
	randomStripeNumInt := randomStripeNum.Int64()
	fmt.Println("stripe num is: ", randomStripeNumInt)

	return &Stripe{
		int(randomStripeNumInt),
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

func generateRandomNumber(pointerLength int) (newRandomNum int64, err error) {
	randomNum, err := rand.Int(rand.Reader, big.NewInt(int64(pointerLength)))
	if err != nil {
		return -1, err
	}
	newRandomNum = randomNum.Int64()
	fmt.Println("randomNum is for pointerItems is ", randomNum)
	return newRandomNum, nil
}
