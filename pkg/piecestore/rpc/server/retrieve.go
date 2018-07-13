// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"errors"
	"io"
	"log"
	"os"

	"storj.io/storj/pkg/piecestore"
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
	path, err := pstore.PathByID(pd.Id, s.PieceStoreDir)
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

	storeFile, err := pstore.RetrieveReader(stream.Context(), pd.Id, pd.Offset, totalToRead, s.PieceStoreDir)
	if err != nil {
		return err
	}

	defer storeFile.Close()

	writer := &StreamWriter{stream: stream}

	for {
		// Receive Bandwidth allocation
		recv, err = stream.Recv()
		if err != nil || recv.Bandwidthallocation == nil {
			log.Println(err)
			return errors.New("Error receiving Bandwidth Allocation data")
		}

		ba := recv.Bandwidthallocation

		// TODO: Should we check if all the variables on ba are good?

		if err = s.verifySignature(ba.Signature); err != nil {
			return err
		}

		sizeToRead := ba.Data.Size
		if ba.Data.Size < 0 {
			sizeToRead = 1024 * 32 // 32 kb
		}

		buf := make([]byte, sizeToRead) // buffer size defined by what is being allocated

		n, err := storeFile.Read(buf)
		if err == io.EOF {
			break
		}
		// Write the buffer to the stream we opened earlier
		_, err = writer.Write(buf[:n])
		if err != nil {
			return err
		}

		if err = s.writeBandwidthAllocToDB(ba); err != nil {
			return err
		}
	}

	log.Println("Successfully retrieved data.")
	return nil
}
