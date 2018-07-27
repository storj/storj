// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"io"
	"log"
	"os"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// RetrieveError is a type of error for failures in Server.Retrieve()
var RetrieveError = errs.Class("retrieve error")

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(stream pb.PieceStoreRoutes_RetrieveServer) error {
	log.Println("Retrieving data...")

	// Receive Signature
	recv, err := stream.Recv()
	if err != nil || recv == nil {
		return RetrieveError.New("Error receiving Piece data")
	}

	pd := recv.GetPieceData()
	log.Printf("ID: %s, Size: %v, Offset: %v\n", pd.GetId(), pd.GetSize(), pd.GetOffset())

	// Get path to data being retrieved
	path, err := pstore.PathByID(pd.GetId(), s.DataDir)
	if err != nil {
		return err
	}

	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Read the size specified
	totalToRead := pd.GetSize()
	fileSize := fileInfo.Size()

	// Read the entire file if specified -1 but make sure we do it from the correct offset
	if pd.GetSize() <= -1 || totalToRead+pd.GetOffset() > fileSize {
		totalToRead = fileSize - pd.GetOffset()
	}

	retrieved, allocated, err := s.retrieveData(stream, pd.GetId(), pd.GetOffset(), totalToRead)
	if err != nil {
		return err
	}

	log.Printf("Successfully retrieved data: Allocated: %v, Retrieved: %v\n", allocated, retrieved)
	return nil
}

func (s *Server) retrieveData(stream pb.PieceStoreRoutes_RetrieveServer, id string, offset, length int64) (retrieved, allocated int64, err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	storeFile, err := pstore.RetrieveReader(ctx, id, offset, length, s.DataDir)
	if err != nil {
		return 0, 0, err
	}

	defer utils.Close(storeFile)

	writer := NewStreamWriter(s, stream)
	am := NewAllocationManager(length)
	var latestBA *pb.BandwidthAllocation

	for am.Used < am.MaxToUse {
		// Receive Bandwidth allocation
		recv, err := stream.Recv()
		if err != nil {
			break
		}

		ba := recv.GetBandwidthallocation()
		baData := ba.GetData()

		// TODO: Validate that Heavy Client agrees to pay up to baData.GetTotal()

		if baData != nil {
			if err = s.verifySignature(ba); err != nil {
				break
			}

			am.AddAllocation(baData.GetSize())
		}

		if ba.GetData().GetTotal() > latestBA.GetData().GetTotal() {
			latestBA = ba
		}

		sizeToRead := am.NextReadSize()

		// Write the buffer to the stream we opened earlier
		n, err := io.CopyN(writer, storeFile, sizeToRead)
		if err != nil {
			break
		}

		if err = am.UseAllocation(n); err != nil {
			break
		}
	}

	// DBError := s.DB.WriteBandwidthAllocToDB(latestBA)
	// if latestBA != nil && DBError != nil {
	// 	log.Println("WriteBandwidthAllocToDB Error:", DBError)
	// }

	return am.Used, am.Allocated, err
}
