// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"io"
	"log"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// OK - Success!
const OK = "OK"

// StoreError is a type of error for failures in Server.Store()
var StoreError = errs.Class("store error")

// Store incoming data using piecestore
func (s *Server) Store(reqStream pb.PieceStoreRoutes_StoreServer) error {
	log.Println("Storing data...")

	// Receive id/ttl
	recv, err := reqStream.Recv()
	if err != nil || recv == nil {
		log.Println(err)
		return StoreError.New("Error receiving Piece Meta Data")
	}

	pd := recv.GetPiecedata()
	log.Printf("ID: %s, TTL: %v\n", pd.GetId(), pd.GetTtl())
	if pd.GetId() == "" {
		return StoreError.New("Invalid Piece ID")
	}

	// If we put in the database first then that checks if the data already exists
	if err = s.DB.AddTTLToDB(pd.GetId(), pd.GetTtl()); err != nil {
		log.Println(err)
		return err
	}

	total, err := s.storeData(reqStream, pd.GetId())
	if err != nil {
		return err
	}

	log.Println("Successfully stored data.")

	return reqStream.SendAndClose(&pb.PieceStoreSummary{Message: OK, TotalReceived: int64(total)})
}

func (s *Server) storeData(stream pb.PieceStoreRoutes_StoreServer, id string) (total int64, err error) {
	// Initialize file for storing data
	storeFile, err := pstore.StoreWriter(id, s.DataDir)
	if err != nil {
		if err = s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}
	defer utils.Close(storeFile)

	reader := NewStreamReader(s, stream)
	total, err = io.Copy(storeFile, reader)

	if err != nil && err != io.EOF {
		log.Println(err)

		if err = s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}

	return total, nil
}
