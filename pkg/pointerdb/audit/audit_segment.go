// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	p "storj.io/storj/pkg/paths"
	pdbclient "storj.io/storj/pkg/pointerdb/pdbclient"
	meta "storj.io/storj/pkg/storage/meta"
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

// ProcessPointer to get randomized pointer
type ProcessPointer interface {
	List(ctx context.Context, startAfter p.Path, limit int) (items []pdbclient.ListItem, more bool, err error)
}

// We'll need to use the pdbclient for requests to pointerdb
type audit struct {
	pdb pdbclient.Client
}

// NewAudit creates a new instance of audit
func NewAudit(pdb pdbclient.Client) ProcessPointer {
	return &audit{pdb: pdb}
}

// List retrevies items from pointerDB so we can process later
func (a *audit) List(ctx context.Context, startAfter p.Path, limit int) (items []pdbclient.ListItem, more bool, err error) {
	return a.pdb.List(ctx, nil, startAfter, nil, true, limit, meta.All)
}
