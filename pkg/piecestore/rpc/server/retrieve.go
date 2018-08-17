// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"io"
	"log"
	"os"

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
	var latestTotal int64

	// Save latest bandwidth allocation even if we fail
	defer func() {
		err := s.DB.WriteBandwidthAllocToDB(latestBA)
		if latestBA != nil && err != nil {
			log.Println("WriteBandwidthAllocToDB Error:", err)
		}
	}()

	for am.Used < length {
		// Receive Bandwidth allocation
		recv, err := stream.Recv()
		if err != nil {
			return am.Used, am.TotalAllocated, err
		}

		ba := recv.GetBandwidthallocation()

		baData := &pb.RenterBandwidthAllocation_Data{}

		if err = proto.Unmarshal(ba.GetData(), baData); err != nil {
			return am.Used, am.TotalAllocated, err
		}

		if err = s.verifySignature(ba); err != nil {
			return am.Used, am.TotalAllocated, err
		}

		am.NewTotal(baData.GetTotal())

		// The bandwidthallocation with the higher total is what I want to earn the most money
		if baData.GetTotal() > latestTotal {
			latestBA = ba
			latestTotal = baData.GetTotal()
		}

		sizeToRead := am.NextReadSize()

		// Write the buffer to the stream we opened earlier
		n, err := io.CopyN(writer, storeFile, sizeToRead)
		if n > 0 {
			if allocErr := am.UseAllocation(n); allocErr != nil {
				log.Println(allocErr)
			}
		}

		if err != nil {
			return am.Used, am.TotalAllocated, err
		}
	}

	return am.Used, am.TotalAllocated, nil
}
