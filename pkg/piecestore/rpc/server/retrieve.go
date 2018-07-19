// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"errors"
	"io"
	"log"
	"os"

	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// Retrieve -- Retrieve data from piecestore and send to client
func (s *Server) Retrieve(stream pb.PieceStoreRoutes_RetrieveServer) error {
	log.Println("Retrieving data...")

	// Receive Signature
	recv, err := stream.Recv()
	if err != nil || recv == nil {
		log.Println(err)
		return errors.New("Error receiving Piece data")
	}

	pd := recv.PieceData
	log.Printf("ID: %s, Size: %v, Offset: %v\n", pd.Id, pd.Size, pd.Offset)

	// Get path to data being retrieved
	path, err := pstore.PathByID(pd.Id, s.DataDir)
	if err != nil {
		return err
	}

	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Read the size specified
	totalToRead := pd.Size
	// Read the entire file if specified -1
	if pd.Size <= -1 {
		totalToRead = fileInfo.Size()
	}

	storeFile, err := pstore.RetrieveReader(stream.Context(), pd.Id, pd.Offset, totalToRead, s.DataDir)
	if err != nil {
		return err
	}

	defer utils.Close(storeFile)

	writer := NewStreamWriter(s, stream)
	var totalRetrieved, totalAllocated int64
	var allocations []int64
	for totalRetrieved < totalToRead {
		// Receive Bandwidth allocation
		recv, err = stream.Recv()
		if err != nil {
			log.Println(err)
			return err
		}

		ba := recv.GetBandwidthallocation()
		baData := ba.GetData()

		if baData != nil {
			if err = s.verifySignature(ba.GetSignature()); err != nil {
				return err
			}

			if err = s.writeBandwidthAllocToDB(ba); err != nil {
				return err
			}

			allocation := baData.GetSize()
			if allocation < 0 {
				allocation = 1024 * 32 // 32 kb
			}

			allocations = append(allocations, allocation)
			totalAllocated += allocation
		}

		if len(allocations) <= 0 {
			continue
		}

		sizeToRead := allocations[len(allocations)-1]

		if sizeToRead > totalToRead-totalRetrieved {
			sizeToRead = totalToRead - totalRetrieved
		}

		buf := make([]byte, sizeToRead) // buffer size defined by what is being allocated
		n, err := storeFile.Read(buf)
		if err == io.EOF {
			break
		}
		// Write the buffer to the stream we opened earlier
		n, err = writer.Write(buf[:n])
		if err != nil {
			return err
		}
		totalRetrieved += int64(n)
		allocations[len(allocations)-1] -= int64(n)

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if allocations[len(allocations)-1] <= 0 {
			allocations = allocations[:len(allocations)-1]
		}
	}

	log.Printf("Successfully retrieved data: Allocated: %v, Retrieved: %v\n", totalAllocated, totalRetrieved)
	return nil
}
