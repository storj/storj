// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"log"

	"storj.io/storj/pkg/utils"
	pb "storj.io/storj/protos/piecestore"
)

// StreamWriter -- Struct for writing piece to server upload stream
type StreamWriter struct {
	server *Server
	stream pb.PieceStoreRoutes_RetrieveServer
	am     *AllocationManager
	ba     *pb.RenterBandwidthAllocation
}

// NewStreamWriter returns a new StreamWriter
func NewStreamWriter(s *Server, stream pb.PieceStoreRoutes_RetrieveServer) *StreamWriter {
	return &StreamWriter{server: s, stream: stream, am: NewAllocationManager()}
}

// Write -- Write method for piece upload to stream for Server.Retrieve
func (sw *StreamWriter) Write(b []byte) (int, error) {
	// Receive Bandwidth allocation
	recv, err := sw.stream.Recv()
	if err != nil {
		return 0, err
	}

	ba := recv.GetBandwidthallocation()
	baData := ba.GetData()
	if baData != nil {
		if err = sw.server.verifySignature(ba); err != nil {
			return 0, err
		}

		sw.am.NewTotal(baData.GetTotal())
	}

	// The bandwidthallocation with the higher total is what I want to earn the most money
	if baData.GetTotal() > sw.ba.GetData().GetTotal() {
		sw.ba = ba
	}

	bLen := len(b)

	sizeToRead := sw.am.NextReadSize()
	if sizeToRead > int64(bLen) {
		sizeToRead = int64(bLen)
	}

	leftoverBytes := int64(bLen) - sizeToRead

	log.Printf("We should be storing the unused bytes: %v\n", leftoverBytes)

	if err := sw.am.UseAllocation(sizeToRead); err != nil {
		return 0, err
	}

	// Write the buffer to the stream we opened earlier
	if err := sw.stream.Send(&pb.PieceRetrievalStream{Size: sizeToRead, Content: b[:sizeToRead]}); err != nil {
		return 0, err
	}

	return bLen, nil
}

// StreamReader is a struct for Retrieving data from server
type StreamReader struct {
	src                 *utils.ReaderSource
	bandwidthAllocation *pb.RenterBandwidthAllocation
}

// NewStreamReader returns a new StreamReader for Server.Store
func NewStreamReader(s *Server, stream pb.PieceStoreRoutes_StoreServer) *StreamReader {
	sr := &StreamReader{}
	sr.src = utils.NewReaderSource(func() ([]byte, error) {

		recv, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		pd := recv.GetPiecedata()
		ba := recv.GetBandwidthallocation()

		if ba != nil {
			if err = s.verifySignature(ba); err != nil {
				return nil, err
			}
		}

		// Update bandwidthallocation to be stored
		if ba.GetData().GetTotal() > sr.bandwidthAllocation.GetData().GetTotal() {
			sr.bandwidthAllocation = ba
		}

		return pd.GetContent(), nil
	})

	return sr
}

// Read -- Read method for piece download from stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}
