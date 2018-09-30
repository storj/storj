// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"reflect"
	"time"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/eestream"
)

//Randomly choose a pointer from pointerdb.
//Library will return a random pointer. Has a super
// user privilege: no one else should be able to randomly
//choose any random pointer. This is purely for auditing,
//and auditing has special privileges.

// ProcessPointer to get and process Pointer data

// Get list pagination of  pointers
// keep track  of last path used so not to duplicate
// randomly select from   that list
// process pointter information for segment
// send that

// Audit  to audit segments
type Audit struct {
	pdb   pdbclient.Client
	store segments.Store
}

// NewAudit creates a new instance of audit
func NewAudit(pdb pdbclient.Client, store segments.Store) *Audit {
	return &Audit{
		pdb:   pdb,
		store: store,
	}
}

// GetList retrevies items from pointerDB so we can process later
func (a *Audit) GetList(ctx context.Context, startAfter paths.Path, limit int) (items []pdbclient.ListItem, more bool, err error) {
	//TODO implement random integer finder
	return a.pdb.List(ctx, nil, startAfter, nil, true, limit, meta.All)
}

// GetPointer returns a pointer
func (a *Audit) GetPointer(ctx context.Context, pointerItem []pdbclient.ListItem) (path paths.Path, pointer *pb.Pointer, err error) {
	path = pointerItem[0].Path
	pointer, err = a.pdb.Get(ctx, path)
	return path, pointer, err
}

//GetSegmentData gets the segment size
func (a *Audit) GetSegmentData(ctx context.Context, path paths.Path) (rr ranger.Ranger, meta segments.Meta, err error) {
	ranger, meta, err := a.store.Get(ctx, path)
	if err != nil {
		return nil, segments.Meta{}, err
	}
	return ranger, meta, nil
}

// GetStripeSize returns the stripe size
func (a *Audit) GetStripeSize(meta segments.Meta, pointer *pb.Pointer) (stripeSize int64) {
	es, err := makeErasureScheme(pointer.GetRemote().GetRedundancy())
	if err != nil {
		return 0
	}

	stripeSize = int64(es.StripeSize())
	return stripeSize
}

// internal function
func makeErasureScheme(rs *pb.RedundancyScheme) (eestream.ErasureScheme, error) {
	fc, err := infectious.NewFEC(int(rs.GetMinReq()), int(rs.GetTotal()))
	if err != nil {
		return nil, err
	}
	es := eestream.NewRSScheme(fc, int(rs.GetErasureShareSize()))
	return es, nil
}

// Get num of stripes per pointer
func getStripeNum(stripeSize int64, segmentSize int64)(stripeNums int64) {
	return segmentSize/stripeSize
}

func generateRandomStripe(stripeNums int64)(stripeNum int64){
	return getItem(stripeNums)
}

// generate random number from length of list
func getItem(i interface{}) (randomInt int64) {
	values := reflect.ValueOf(i)
	rand.Seed(time.Now().UnixNano())
	num := int64(rand.Intn(values.Len()))
	return num
}
