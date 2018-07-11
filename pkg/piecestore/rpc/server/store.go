// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"errors"
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

	// Receive id/ttl
	recv, err := reqStream.Recv()
	if err != nil || recv == nil {
		log.Println(err)
		return errors.New("Error receiving Piece Meta Data")
	}

	pd := recv.Piecedata
	log.Printf("ID: %s, TTL: %v\n", pd.Id, pd.Ttl)

	// If we put in the database first then that checks if the data already exists
	if err = s.DB.AddTTLToDB(pd.Id, pd.Ttl); err != nil {
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

func (s *Server) storeData(stream pb.PieceStoreRoutes_StoreServer, id string) (total int64, err error) {
	// Initialize file for storing data
	storeFile, err := pstore.StoreWriter(id, s.PieceStoreDir)
	if err != nil {
		if err = s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}
	defer storeFile.Close()

	reader := s.NewStreamReader(stream)

	for {
		// Receive Bandwidth allocation
		recv, err := stream.Recv()
		if err != nil || recv.Bandwidthallocation == nil {
			break
		}

		ba := recv.Bandwidthallocation

		// TODO: Should we check if all the variables on ba are good?

		// TODO: verify signature
		if err = s.verifySignature(ba.Signature); err != nil {
			break
		}

		// TODO: Save bandwidthallocation to DB
		if err = s.writeBandwidthAllocToDB(ba); err != nil {
			break
		}

		buf := make([]byte, ba.Data.Size) // buffer size defined by what is being allocated

		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		// Write the buffer to the stream we opened earlier
		n, err = storeFile.Write(buf[:n])
		if err != nil {
			break
		}

		total += int64(n)
	}

	// total, err := io.Copy(storeFile, reader)
	if err != nil && err != io.EOF {
		log.Println(err)

		if err = s.deleteByID(id); err != nil {
			log.Printf("Failed on deleteByID in Store: %s", err.Error())
		}

		return 0, err
	}

	return total, nil
}
