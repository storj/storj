// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
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
	if err != nil {
		return err
	}

	// TODO: verify signature
	if err := s.verifySignature(recv.Signature); err != nil {
		return err
	}

	// get bandwidth allocation
	recv, err = stream.Recv()
	if err != nil {
		return err
	}

	ba := recv.Bandwidthallocation
	log.Printf("Payer: %s, Client: %s, Size: %v\n", ba.Payer, ba.Client, ba.Size)

	// get Piecedata
	recv, err = stream.Recv()
	if err != nil {
		return err
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
	_, err = io.Copy(writer, storeFile)
	if err != nil {
		return err
	}

	// TODO: Save bandwidthallocation to DB

	log.Println("Successfully retrieved data.")
	return nil
}
