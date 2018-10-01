// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/meta"
)

// Audit to audit segments
type Audit struct {
	pdb pdbclient.Client
	r   *rand.Rand
}

// NewAudit creates a new instance of audit
func NewAudit(pdb pdbclient.Client, r *rand.Rand) *Audit {
	return &Audit{
		pdb: pdb,
		r:   r,
	}
}

// Stripe is a struct that contains stripe info
type Stripe struct {
	stripe int
}

// NextStripe returns a random stripe to be audited
func (a *Audit) NextStripe(ctx context.Context, startAfter paths.Path, limit int) (stripe *Stripe, err error) {
	// retreive a random list of pointers
	pointerItems, _, err := a.pdb.List(ctx, nil, startAfter, nil, true, limit, meta.None)
	if err != nil {
		return nil, err
	}

	randomNum := a.getItem(pointerItems)
	pointerItem := pointerItems[randomNum]

	// get a pointer
	path := pointerItem.Path
	pointer, err := a.pdb.Get(ctx, path)

	if err != nil {
		return nil, err
	}

	// create the erasure scheme so we can get the stripe size
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return nil, err
	}

	// get random stripe
	stripeSize := int64(es.StripeSize())
	stripeNum := a.getItem(int(pointer.GetSize() / stripeSize))

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

// generate random number from length of list
func (a *Audit) getItem(i interface{}) (randomInt int) {
	switch i := i.(type) {
	case int:
		num := int(a.r.Intn(i))
		return num

	case []pdbclient.ListItem:
		num := int(a.r.Intn(len(i)))
		return num

	default:
		panic(fmt.Sprintf("unexpected type: %T", i))
	}
}
