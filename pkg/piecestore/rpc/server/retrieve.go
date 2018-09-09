// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/sync2"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// RetrieveError is a type of error for failures in Server.Retrieve()
var RetrieveError = errs.Class("retrieve error")

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(stream pb.PieceStoreRoutes_RetrieveServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	// Receive Signature
	recv, err := stream.Recv()
	if err != nil {
		return RetrieveError.Wrap(err)
	}
	if recv == nil {
		return RetrieveError.New("error receiving piece data")
	}

	pd := recv.GetPieceData()
	if pd == nil {
		return RetrieveError.New("PieceStore message is nil")
	}

	log.Printf("Retrieving %s...", pd.GetId())

	// Get path to data being retrieved
	path, err := pstore.PathByID(pd.GetId(), s.DataDir)
	if err != nil {
		return err
	}

	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return RetrieveError.Wrap(err)
	}

	// Read the size specified
	totalToRead := pd.GetSize()
	fileSize := fileInfo.Size()

	// Read the entire file if specified -1 but make sure we do it from the correct offset
	if pd.GetSize() <= -1 || totalToRead+pd.GetOffset() > fileSize {
		totalToRead = fileSize - pd.GetOffset()
	}

	retrieved, allocated, err := s.retrieveData(ctx, stream, pd.GetId(), pd.GetOffset(), totalToRead)
	if err != nil {
		return err
	}

	log.Printf("Successfully retrieved %s: Allocated: %v, Retrieved: %v\n", pd.GetId(), allocated, retrieved)
	return nil
}

func (s *Server) retrieveData(ctx context.Context, stream pb.PieceStoreRoutes_RetrieveServer, id string, offset, length int64) (retrieved, allocated int64, err error) {
	defer mon.Task()(&ctx)(&err)

	storeFile, err := pstore.RetrieveReader(ctx, id, offset, length, s.DataDir)
	if err != nil {
		return 0, 0, err
	}

	defer utils.LogClose(storeFile)

	writer := NewStreamWriter(s, stream)
	allocationTracking := sync2.NewThrottle()
	totalAllocated := int64(0)

	// Bandwidth Allocation recv loop
	go func() {
		var lastTotal int64
		var lastAllocation *pb.RenterBandwidthAllocation
		defer func() {
			if lastAllocation == nil {
				return
			}
			err := s.DB.WriteBandwidthAllocToDB(lastAllocation)
			if err != nil {
				// TODO: handle error properly
				log.Println("WriteBandwidthAllocToDB Error:", err)
			}
		}()

		for {
			recv, err := stream.Recv()
			if err != nil {
				allocationTracking.Fail(err)
				return
			}

			alloc := recv.GetBandwidthallocation()
			allocData := &pb.RenterBandwidthAllocation_Data{}
			if err = proto.Unmarshal(alloc.GetData(), allocData); err != nil {
				allocationTracking.Fail(err)
				return
			}

			if err = s.verifySignature(ctx, alloc); err != nil {
				allocationTracking.Fail(err)
				return
			}

			if lastTotal > allocData.GetTotal() {
				allocationTracking.Fail(fmt.Errorf("got lower allocation was %v got %v", lastTotal, allocData.GetTotal()))
			}
			atomic.StoreInt64(&totalAllocated, allocData.GetTotal())

			err = allocationTracking.Produce(allocData.GetTotal() - lastTotal)
			if err != nil {
				return
			}

			lastAllocation = alloc
			lastTotal = allocData.GetTotal()
		}
	}()

	// Data send loop
	messageSize := int64(8 << 10)
	used := int64(0)

	for {
		nextMessageSize, err := allocationTracking.ConsumeOrWait(messageSize)
		if err != nil {
			allocationTracking.Fail(err)
			break
		}

		used += nextMessageSize
		n, err := io.CopyN(writer, storeFile, nextMessageSize)
		if err != nil {
			allocationTracking.Fail(err)
			break
		}
		// correct errors when needed
		if n != nextMessageSize {
			allocationTracking.Produce(nextMessageSize - n)
			used -= nextMessageSize - n
		}
	}

	// TODO: handle errors
	// _ = stream.Close()

	return used, atomic.LoadInt64(&totalAllocated), allocationTracking.Err()
}
