// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
)

// OK - Success!
const OK = "OK"

// StoreError is a type of error for failures in Server.Store()
var StoreError = errs.Class("store error")

// Store stores incoming data using piecestore
func (s *Server) Store(reqStream pb.PieceStoreRoutes_StoreServer) (err error) {
	ctx := reqStream.Context()
	defer mon.Task()(&ctx)(&err)
	// Receive id/ttl
	recv, err := reqStream.Recv()
	if err != nil {
		return StoreError.Wrap(err)
	}
	if recv == nil {
		return StoreError.New("error receiving Piece metadata")
	}

	pd := recv.GetPieceData()
	if pd == nil {
		return StoreError.New("PieceStore message is nil")
	}

	s.log.Debug("Storing", zap.String("Piece ID", fmt.Sprint(pd.GetId())))

	if pd.GetId() == "" {
		return StoreError.New("piece ID not specified")
	}

	rba := recv.GetBandwidthAllocation()
	if rba == nil {
		return StoreError.New("Order message is nil")
	}

	pba := rba.PayerAllocation
	if pb.Equal(&pba, &pb.OrderLimit{}) {
		return StoreError.New("OrderLimit message is empty")
	}

	id, err := getNamespacedPieceID([]byte(pd.GetId()), pba.SatelliteId.Bytes())
	if err != nil {
		return err
	}
	err = s.storeData(ctx, pd, reqStream, id)
	if err != nil {
		return err
	}

	s.log.Info("Successfully stored", zap.String("Piece ID", fmt.Sprint(pd.GetId())))

	return nil
}

func (s *Server) storeData(ctx context.Context, pd *pb.PieceStore_PieceData, stream pb.PieceStoreRoutes_StoreServer, id string) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Delete data if we error
	defer func() {
		if err != nil && err != io.EOF {
			if deleteErr := s.deleteByID(id); deleteErr != nil {
				s.log.Error("Failed on deleteByID in Store", zap.Error(deleteErr))
			}
		}
	}()

	// Initialize file for storing data
	storeFile, err := s.storage.Writer(id)
	if err != nil {
		return err
	}

	defer func() { err = errs.Combine(err, storeFile.Close()) }()

	bwUsed, err := s.DB.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return err
	}
	spaceUsed, err := s.DB.SumTTLSizes()
	if err != nil {
		return err
	}
	bwLeft := s.totalBwAllocated - bwUsed
	spaceLeft := s.totalAllocated - spaceUsed
	reader := NewStreamReader(s, stream, bwLeft, spaceLeft)

	total, err := io.Copy(storeFile, reader)
	if err != nil && err != io.EOF {
		return err
	}

	if reader.done == nil {
		return StoreError.New("no uplink piece hash for verification")
	}

	// TODO verify incomming piece hash signature

	// TODO save to DB in goroutine
	err = s.DB.WriteBandwidthAllocToDB(reader.bandwidthAllocation)
	if err != nil {
		return StoreError.Wrap(err)
	}

	if err = s.DB.AddTTL(id, pd.GetExpirationUnixSec(), total); err != nil {
		return StoreError.New("failed to write piece meta data to database: %v", err)
	}

	hash := reader.hash.Sum(nil)
	signedHash := &pb.SignedHash{Hash: hash}
	err = auth.SignMessage(signedHash, *s.identity)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&pb.PieceStoreSummary{
		Message:       OK,
		TotalReceived: total,
		SignedHash:    signedHash,
	})
}
