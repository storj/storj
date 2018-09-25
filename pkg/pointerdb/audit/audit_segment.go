// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/meta"
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
	pdb pdbclient.Client
	psc client.PSClient
}

// NewAudit creates a new instance of audit
func NewAudit(pdb pdbclient.Client, psc client.PSClient) *Audit {
	return &Audit{
		pdb: pdb,
		psc: psc,
	}
}

// List retrevies items from pointerDB so we can process later
func (a *Audit) List(ctx context.Context, startAfter paths.Path, limit int) (items []pdbclient.ListItem, more bool, err error) {
	//TODO get a  random  segment from  the LIST  and return one segments
	return a.pdb.List(ctx, nil, startAfter, nil, true, limit, meta.All)
}

// GetPieceInfo gets the derived pieceID
func (a *Audit) GetPieceInfo(ctx context.Context, path paths.Path) (derivedPieceID client.PieceID, pieceSize int64, err error) {
	pointer, err := a.pdb.Get(ctx, path)
	if err != nil {
		return "", 0, err
	}
	remoteSegment := pointer.GetRemote()
	remotePieceID := remoteSegment.GetPieceId()
	remotePieces := remoteSegment.GetRemotePieces()
	// TODO create a  random generator for a list
	nodeID := remotePieces[0].GetNodeId()

	//type cast to client.PieceID
	var pieceID = client.PieceID(remotePieceID)

	derivedPieceID, err = pieceID.Derive([]byte(nodeID))
	if err != nil {
		return "", 0, err
	}

	fmt.Println("derived piece id in audit segment: ", derivedPieceID)

	pieceSummary, err := a.psc.Meta(ctx, derivedPieceID)
	if err != nil {
		fmt.Println("error in piecesummary")
		return "", 0, err
	}

	return derivedPieceID, pieceSummary.GetSize(), nil
}

// GetStripe retreives a strip from PSClients
// for now pbwa is {} - not implemented yet
func (a *Audit) GetStripe(ctx context.Context, pieceID client.PieceID, size int64, pbwa *pb.PayerBandwidthAllocation) (ranger ranger.Ranger, err error) {
	ranger, err = a.psc.Get(ctx, pieceID, size, pbwa)
	if err != nil {
		return nil, err
	}
	fmt.Println("this is the ranger ac: ", ranger)
	return ranger, nil
}

// generate random number from length of list
func getItem(i interface{}) (randomInt int) {
	values := reflect.ValueOf(i)
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(values.Len())
}
