// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"io"
	"log"

	"storj.io/storj/pkg/piecestore"
	pb "storj.io/storj/protos/piecestore"
)

// OK - Success!
const OK = "OK"

// Store incoming data using piecestore
func (s *Server) Store(reqStream pb.PieceStoreRoutes_StoreServer) error {
	log.Println("Storing data...")

	// Receive Signature
	recv, err := reqStream.Recv()
	if err != nil {
		return err
	}

	// TODO: verify signature
	if err := s.verifySignature(recv.Signature); err != nil {
		return err
	}

	// Receive payer/client/size
	recv, err = reqStream.Recv()
	if err != nil {
		return err
	}

	ba := recv.Bandwidthallocation
	log.Printf("Payer: %s, Client: %s, Size: %v\n", ba.Payer, ba.Client, ba.Size)

	// Receive id/ttl
	recv, err = reqStream.Recv()
	if err != nil {
		return err
	}

	pd := recv.Piecedata
	log.Printf("ID: %s, TTL: %v\n", pd.Id, pd.Ttl)

	// If we put in the database first then that checks if the data already exists
	if err := s.DB.AddTTLToDB(pd.Id, pd.Ttl); err != nil {
		log.Println(err)
		return err
	}

	// TODO: Save bandwidthAllocation and signature to DB

	total, err := s.storeData(reqStream, pd.Id)
	if err != nil {
		return err
	}

	log.Println("Successfully stored data.")

	return reqStream.SendAndClose(&pb.PieceStoreSummary{Message: OK, TotalReceived: int64(total)})
}

func (s *Server) storeData(stream pb.PieceStoreRoutes_StoreServer, id string) (int64, error) {
	// Initialize file for storing data
	storeFile, err := pstore.StoreWriter(id, s.PieceStoreDir)
	if err != nil {
		if err := s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}
	defer storeFile.Close()

	reader := NewStreamReader(stream)
	total, err := io.Copy(storeFile, reader)
	if err != nil {
		if err := s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}

	return total, nil
}
