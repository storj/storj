// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"reflect"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
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

// List retrevies items from pointerDB so we can process later
func (a *Audit) List(ctx context.Context, startAfter paths.Path, limit int) (items []pdbclient.ListItem, more bool, err error) {
	//TODO implement random integer finder
	return a.pdb.List(ctx, nil, startAfter, nil, true, limit, meta.All)
}

func (a *Audit) GetSegmentSize(ctx context.Context, path paths.Path) {

}

// func (a *Audit) GetSize(ctx context.Context, meta segments.Meta) {

// }

// generate random number from length of list
func getItem(i interface{}) (randomInt int) {
	values := reflect.ValueOf(i)
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(values.Len())
}
