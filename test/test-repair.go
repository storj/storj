// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"log"

	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage/redis"
)

const (
	apiKey  = "abc123"
	qPath   = "redis://127.0.0.1:6378?db=1&password=abc123"
	pdbAddr = "127.0.0.1:7778"
	fPath   = "bucket/big-testfile"
	rootKey = "highlydistributedridiculouslyresilient"
)

var ctx = context.Background()

func main() {
	client, err := redis.NewClientFrom(qPath)
	if err != nil {
		log.Fatal(err)
	}

	q := queue.NewQueue(client)
	key := new(storj.Key)
	copy(key[:], rootKey)

	encPath, err := streams.EncryptAfterBucket(fPath, key)
	if err != nil {
		log.Fatal(err)
	}

	finalPath := storj.JoinPaths("l", encPath)

	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		log.Fatal(err)
	}

	identity, err := ca.NewIdentity()
	if err != nil {
		log.Fatal(err)
	}

	pdb, err := pdbclient.NewClient(identity, pdbAddr, apiKey)
	if err != nil {
		log.Fatal(err)
	}

	pr, _, err := pdb.Get(ctx, finalPath)
	if err != nil {
		log.Fatal(err)
	}

	seg := pr.GetRemote()
	pieces := seg.GetRemotePieces()

	var lostPieces []int32
	for i := 0; i < 5; i++ {
		lostPieces = append(lostPieces, pieces[i].GetPieceNum())
	}

	err = q.Enqueue(&pb.InjuredSegment{
		Path:       finalPath,
		LostPieces: lostPieces,
	})
	if err != nil {
		log.Fatal(err)
	}

	//TODO check updated pointer data against original data
}
