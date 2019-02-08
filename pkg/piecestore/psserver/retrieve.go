// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"

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
		fmt.Println("RECEIVE ERR", err)
		return RetrieveError.Wrap(err)
	}
	if recv == nil {
		return RetrieveError.New("error receiving piece data")
	}

	authorization := recv.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		fmt.Println("AUTHORIZATION ERR", err)
		return ServerError.Wrap(err)
	}

	pd := recv.GetPieceData()
	if pd == nil {
		fmt.Println("GET PIECEDATA", err)
		return RetrieveError.New("PieceStore message is nil")
	}

	id, err := getNamespacedPieceID([]byte(pd.GetId()), getNamespace(authorization))
	if err != nil {
		fmt.Println("GET NAMESPACED PIECE ID", err)
		return err
	}
	s.log.Debug("Retrieving",
		zap.String("Piece ID", id),
		zap.Int64("Offset", pd.GetOffset()),
		zap.Int64("Size", pd.GetPieceSize()),
	)
	// Get path to data being retrieved
	path, err := s.storage.PiecePath(id)
	if err != nil {
		fmt.Println("PIECE PATH", err)
		return err
	}
	// Verify that the path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Println("STAT", err)
		return RetrieveError.Wrap(err)
	}
	// Read the size specified
	totalToRead := pd.GetPieceSize()
	fileSize := fileInfo.Size()
	// Read the entire file if specified -1 but make sure we do it from the correct offset
	if pd.GetPieceSize() <= -1 || totalToRead+pd.GetOffset() > fileSize {
		totalToRead = fileSize - pd.GetOffset()
	}
	retrieved, allocated, err := s.retrieveData(ctx, stream, id, pd.GetOffset(), totalToRead)
	if err != nil {
		fmt.Println("RETRIEVE DATA", err)
		return err
	}
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
		fmt.Println("READER", err)
		return 0, 0, RetrieveError.Wrap(err)
	}

	defer func() {
		err2 := RetrieveError.Wrap(storeFile.Close())
		fmt.Println("CLOSING FILE", err2)
		err = errs.Combine(err, err2)
	}()

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
			dbErr := s.DB.WriteBandwidthAllocToDB(lastAllocation)
			fmt.Println("WriteBandwidthAllocToDB", dbErr)
			err = errs.Combine(err, RetrieveError.Wrap(dbErr))
		}()

		for {
			recv, err := stream.Recv()
			if err != nil {
				fmt.Println("RECV WWW", err)
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			rba := recv.BandwidthAllocation
			if err = s.verifySignature(stream.Context(), rba); err != nil {
				fmt.Println("Ret verifySignature", err)
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			pba := rba.PayerAllocation
			if err = s.verifyPayerAllocation(&pba, "GET"); err != nil {
				fmt.Println("Ret verifyPayerAllocation", err)
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			//todo: figure out why this fails tests
			// if rba.Total > pba.MaxSize {
			// 	allocationTracking.Fail(fmt.Errorf("attempt to send more data than allocation %v got %v", rba.Total, pba.MaxSize))
			// 	return
			// }
			if lastTotal > rba.Total {
				fmt.Println("Ret total", err)
				allocationTracking.Fail(fmt.Errorf("got lower allocation was %v got %v", lastTotal, rba.Total))
				return RetrieveError.Wrap(err)
			}
			atomic.StoreInt64(&totalAllocated, rba.Total)
			if err = allocationTracking.Produce(rba.Total - lastTotal); err != nil {
				fmt.Println("produce", err)
				return RetrieveError.Wrap(err)
			}
			lastAllocation = rba
			lastTotal = rba.Total
		}
	})

	// Data send loop
	messageSize := int64(32 * memory.KiB)
	used := int64(0)
	group.Go(func() error {
		for used < length {
			nextMessageSize, err := allocationTracking.ConsumeOrWait(messageSize)
			if err != nil {
				fmt.Println("Ret consume or wait", err)
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
			toCopy := nextMessageSize
			if length-used < nextMessageSize {
				toCopy = length - used
			}
			used += nextMessageSize
			n, err := io.CopyN(writer, storeFile, toCopy)
			// correct errors when needed
			if n != nextMessageSize {
				if err := allocationTracking.Produce(nextMessageSize - n); err != nil {
					fmt.Println("Ret produce 222", err)
					return RetrieveError.Wrap(err)
				}
				used -= nextMessageSize - n
			}
			if used >= length && err == io.EOF {
				return nil
			}
			if err != nil {
				fmt.Println("break break", err)
				// break on error
				allocationTracking.Fail(RetrieveError.Wrap(err))
				return RetrieveError.Wrap(err)
			}
		}
		return nil
	})

	errx := group.Wait()
	// write to bandwidth usage table
	bwUsedErr := s.DB.AddBandwidthUsed(used)
	return used, atomic.LoadInt64(&totalAllocated), errs.Combine(errx, allocationTracking.Err(), RetrieveError.Wrap(bwUsedErr))
}
