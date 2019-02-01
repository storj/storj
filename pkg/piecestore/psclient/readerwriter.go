// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"fmt"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
)

// StreamWriter creates a StreamWriter for writing data to the piece store server
type StreamWriter struct {
	stream       pb.PieceStoreRoutes_StoreClient
	signer       *PieceStore // We need this for signing
	totalWritten int64
	pba          *pb.PayerBandwidthAllocation
}

// Write Piece data to a piece store server upload stream
func (s *StreamWriter) Write(b []byte) (int, error) {
	updatedAllocation := s.totalWritten + int64(len(b))
	rba := &pb.RenterBandwidthAllocation{
		PayerAllocation: *s.pba,
		Total:           updatedAllocation,
		StorageNodeId:   s.signer.remoteID,
	}
	err := auth.SignMessage(rba, *s.signer.selfID)
	if err != nil {
		return 0, err
	}
	msg := &pb.PieceStore{
		PieceData:           &pb.PieceStore_PieceData{Content: b},
		BandwidthAllocation: rba,
	}
	s.totalWritten = updatedAllocation
	// Second we send the actual content
	if err := s.stream.Send(msg); err != nil {
		return 0, fmt.Errorf("%v.Send() = %v", s.stream, err)
	}
	return len(b), nil
}

// Close the piece store Write Stream
func (s *StreamWriter) Close() error {
	reply, err := s.stream.CloseAndRecv()
	if err != nil {
		return err
	}

	zap.S().Infof("Stream close and recv summary: %v", reply)

	return nil
}

// StreamReader is a struct for reading piece download stream from server
type StreamReader struct {
	pendingAllocs *sync2.Throttle
	client        *PieceStore
	stream        pb.PieceStoreRoutes_RetrieveClient
	src           *utils.ReaderSource
	downloaded    int64
	allocated     int64
	size          int64
}

// NewStreamReader creates a StreamReader for reading data from the piece store server
func NewStreamReader(client *PieceStore, stream pb.PieceStoreRoutes_RetrieveClient, pba *pb.PayerBandwidthAllocation, size int64) *StreamReader {
	sr := &StreamReader{
		pendingAllocs: sync2.NewThrottle(),
		client:        client,
		stream:        stream,
		size:          size,
	}
	// TODO: make these flag/config-file configurable
	trustLimit := int64(client.bandwidthMsgSize * 64)
	sendThreshold := int64(client.bandwidthMsgSize * 8)

	// Send signed allocations to the piece store server
	go func() {
		// TODO: make this flag/config-file configurable
		trustedSize := int64(client.bandwidthMsgSize * 8)

		// Allocate until we've reached the file size
		for sr.allocated < size {
			allocate := trustedSize
			if sr.allocated+trustedSize > size {
				allocate = size - sr.allocated
			}
			rba := &pb.RenterBandwidthAllocation{
				PayerAllocation: *pba,
				Total:           sr.allocated + allocate,
				StorageNodeId:   sr.client.remoteID,
			}
			err := auth.SignMessage(rba, *client.selfID)
			if err != nil {
				sr.pendingAllocs.Fail(err)
			}
			msg := &pb.PieceRetrieval{BandwidthAllocation: rba}

			if err = stream.Send(msg); err != nil {
				sr.pendingAllocs.Fail(err)
				return
			}

			sr.allocated += trustedSize

			if err = sr.pendingAllocs.ProduceAndWaitUntilBelow(allocate, sendThreshold); err != nil {
				return
			}

			// Speed up retrieval as server gives us more data
			trustedSize *= 2
			if trustedSize > trustLimit {
				trustedSize = trustLimit
			}
		}
	}()

	sr.src = utils.NewReaderSource(func() ([]byte, error) {
		resp, err := stream.Recv()
		if err != nil {
			sr.pendingAllocs.Fail(err)
			return nil, err
		}

		sr.downloaded += int64(len(resp.GetContent()))

		err = sr.pendingAllocs.Consume(int64(len(resp.GetContent())))
		if err != nil {
			sr.pendingAllocs.Fail(err)
			return resp.GetContent(), err
		}

		return resp.GetContent(), nil
	})

	return sr
}

// Read Piece data from piece store server download stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}

// Close the piece store server Read Stream
func (s *StreamReader) Close() error {
	return utils.CombineErrors(
		s.stream.CloseSend(),
		s.client.Close(),
	)
}
