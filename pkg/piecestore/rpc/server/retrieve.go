// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/piecestore"
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
	am := NewAllocationManager()
	var latestBA *pb.RenterBandwidthAllocation
	var mtx sync.Mutex
	cond := sync.NewCond(&mtx)

	errChan := make(chan error)

	// Save latest bandwidth allocation even if we fail
	defer func() {
		baWriteErr := s.DB.WriteBandwidthAllocToDB(latestBA)
		if latestBA != nil && baWriteErr != nil {
			log.Println("WriteBandwidthAllocToDB Error:", baWriteErr)
		}
	}()

	// Bandwidth Allocation recv loop
	go func() {
		var latestTotal int64

		for am.Used < length {
			recv, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}

			ba := recv.GetBandwidthallocation()

			baData := &pb.RenterBandwidthAllocation_Data{}

			if err = proto.Unmarshal(ba.GetData(), baData); err != nil {
				errChan <- err
				return
			}

			fmt.Println("Receiving ba: ", baData)
			if err = s.verifySignature(ctx, ba); err != nil {
				errChan <- err
				return
			}

			mtx.Lock() // Lock when updating bandwidth allocation
			am.NewTotal(baData.GetTotal())
			cond.Signal() // Signal the data send loop that it has available bandwidth
			mtx.Unlock()

			if baData.GetTotal() > latestTotal {
				latestBA = ba
				latestTotal = baData.GetTotal()
			}
		}

		errChan <- nil
	}()

	// Data send loop
	go func() {
		for am.Used < length {

			cond.L.Lock()
			for am.NextReadSize() <= 0 {
				cond.Wait() // Wait for the bandwidth loop to get a new bandwidth allocation
			}
			cond.L.Unlock()

			sizeToRead := am.NextReadSize()

			// Write the buffer to the stream we opened earlier
			n, err := io.CopyN(writer, storeFile, sizeToRead)
			if n > 0 {
				mtx.Lock()
				if allocErr := am.UseAllocation(n); allocErr != nil {
					log.Println(allocErr)
				}
				mtx.Unlock()
			}

			if err != nil {
				errChan <- err
				return
			}
		}

		errChan <- nil
	}()

	err = <-errChan
	return am.Used, am.TotalAllocated, err
}
