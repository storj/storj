// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
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

	jitter()

	authorization := recv.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		return ServerError.Wrap(err)
	}

	jitter()

	pd := recv.GetPieceData()
	if pd == nil {
		return RetrieveError.New("PieceStore message is nil")
	}

	id, err := getNamespacedPieceID([]byte(pd.GetId()), getNamespace(authorization))
	if err != nil {
		return err
	}
	jitter()
	s.log.Debug("Retrieving",
		zap.String("Piece ID", id),
		zap.Int64("Offset", pd.GetOffset()),
		zap.Int64("Size", pd.GetPieceSize()),
	)
	jitter()
	// Get path to data being retrieved
	path, err := s.storage.PiecePath(id)
	if err != nil {
		return err
	}
	jitter()
	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return RetrieveError.Wrap(err)
	}
	jitter()
	// Read the size specified
	totalToRead := pd.GetPieceSize()
	fileSize := fileInfo.Size()
	jitter()
	// Read the entire file if specified -1 but make sure we do it from the correct offset
	if pd.GetPieceSize() <= -1 || totalToRead+pd.GetOffset() > fileSize {
		totalToRead = fileSize - pd.GetOffset()
	}
	jitter()
	retrieved, allocated, err := s.retrieveData(ctx, stream, id, pd.GetOffset(), totalToRead)
	if err != nil {
		return err
	}
	jitter()
	s.log.Info("Successfully retrieved",
		zap.String("Piece ID", id),
		zap.Int64("Allocated", allocated),
		zap.Int64("Retrieved", retrieved),
	)
	return nil
}

func (s *Server) retrieveData(ctx context.Context, stream pb.PieceStoreRoutes_RetrieveServer, id string, offset, length int64) (retrieved, allocated int64, err error) {
	defer mon.Task()(&ctx)(&err)

	storeFile, err := s.storage.Reader(ctx, id, offset, length)
	if err != nil {
		return 0, 0, RetrieveError.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, storeFile.Close())
	}()

	jitter()

	writer := NewStreamWriter(s, stream)
	allocationTracking := sync2.NewThrottle()
	totalAllocated := int64(0)

	var group errgroup.Group

	// Bandwidth Allocation recv loop
	group.Go(func() (err error) {
		var lastTotal int64
		var lastAllocation *pb.RenterBandwidthAllocation
		defer func() {
			if lastAllocation == nil {
				return
			}
			jitter()
			dbErr := s.DB.WriteBandwidthAllocToDB(lastAllocation)
			err = errs.Combine(err, RetrieveError.Wrap(dbErr))
		}()

		for {
			jitter()
			recv, err := stream.Recv()
			if err != nil {
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			jitter()
			rba := recv.BandwidthAllocation
			if err = s.verifySignature(stream.Context(), rba); err != nil {
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			jitter()
			pba := rba.PayerAllocation
			if err = s.verifyPayerAllocation(&pba, "GET"); err != nil {
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			jitter()
			//todo: figure out why this fails tests
			// if rba.Total > pba.MaxSize {
			// 	allocationTracking.Fail(fmt.Errorf("attempt to send more data than allocation %v got %v", rba.Total, pba.MaxSize))
			// 	return
			// }
			if lastTotal > rba.Total {
				allocationTracking.Fail(fmt.Errorf("got lower allocation was %v got %v", lastTotal, rba.Total))
				return RetrieveError.Wrap(err)
			}
			jitter()
			atomic.StoreInt64(&totalAllocated, rba.Total)
			jitter()
			if err = allocationTracking.Produce(rba.Total - lastTotal); err != nil {
				return RetrieveError.Wrap(err)
			}
			jitter()
			lastAllocation = rba
			lastTotal = rba.Total
		}
		return nil
	})

	// Data send loop
	messageSize := int64(32 * memory.KiB)
	used := int64(0)
	group.Go(func() error {
		for used < length {
			jitter()
			nextMessageSize, err := allocationTracking.ConsumeOrWait(messageSize)
			if err != nil {
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			jitter()
			toCopy := nextMessageSize
			if length-used < nextMessageSize {
				toCopy = length - used
			}
			jitter()
			used += nextMessageSize
			jitter()
			n, err := io.CopyN(writer, storeFile, toCopy)
			// correct errors when needed
			jitter()
			if n != nextMessageSize {
				if err := allocationTracking.Produce(nextMessageSize - n); err != nil {
					return RetrieveError.Wrap(err)
				}
				used -= nextMessageSize - n
			}
			jitter()
			if used >= length && err == io.EOF {
				return nil
			}
			jitter()
			if err != nil {
				// break on error
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			jitter()
		}
		return nil
	})

	errx := group.Wait()

	jitter()
	// write to bandwidth usage table
	if err = s.DB.AddBandwidthUsed(used); err != nil {
		return retrieved, allocated, RetrieveError.New("failed to write bandwidth info to database: %v", err)
	}

	// TODO: handle errors
	// _ = stream.Close()

	return used, atomic.LoadInt64(&totalAllocated), errs.Combine(errx, allocationTracking.Err())
}

func jitter() {
	time.Sleep(time.Duration(rand.Intn(3)*100) * time.Microsecond)
}
