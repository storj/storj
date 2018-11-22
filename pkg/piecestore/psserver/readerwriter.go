// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/utils"
)

// StreamWriter -- Struct for writing piece to server upload stream
type StreamWriter struct {
	server *Server
	stream pb.PieceStoreRoutes_RetrieveServer
}

// NewStreamWriter returns a new StreamWriter
func NewStreamWriter(s *Server, stream pb.PieceStoreRoutes_RetrieveServer) *StreamWriter {
	return &StreamWriter{server: s, stream: stream}
}

// Write -- Write method for piece upload to stream for Server.Retrieve
func (s *StreamWriter) Write(b []byte) (int, error) {
	// Write the buffer to the stream we opened earlier
	if err := s.stream.Send(&pb.PieceRetrievalStream{PieceSize: int64(len(b)), Content: b}); err != nil {
		return 0, err
	}

	return len(b), nil
}

// StreamReader is a struct for Retrieving data from server
type StreamReader struct {
	src                 *utils.ReaderSource
	bandwidthAllocation *pb.RenterBandwidthAllocation
	currentTotal        int64
}

// NewStreamReader returns a new StreamReader for Server.Store
func NewStreamReader(s *Server, stream pb.PieceStoreRoutes_StoreServer) *StreamReader {
	sr := &StreamReader{}
	sr.src = utils.NewReaderSource(func() ([]byte, error) {

		recv, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		pd := recv.GetPieceData()
		ba := recv.GetBandwidthAllocation()

		if ba != nil {
			if err = s.verifySignature(stream.Context(), ba); err != nil {
				return nil, err
			}

			deserializedData := &pb.RenterBandwidthAllocation_Data{}
			err = proto.Unmarshal(ba.GetData(), deserializedData)
			if err != nil {
				return nil, err
			}

			// Update bandwidthallocation to be stored
			if deserializedData.GetTotal() > sr.currentTotal {
				sr.bandwidthAllocation = ba
				sr.currentTotal = deserializedData.GetTotal()
			}
		}

		return pd.GetContent(), nil
	})

	return sr
}

// Read -- Read method for piece download from stream
func (s *StreamReader) Read(b []byte) (int, error) {
	return s.src.Read(b)
}
