// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	pb "storj.io/storj/protos/piecestore"
)

func TestPieceRanger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		errString            string
	}{
		{"", 0, 0, 0, "", ""},
		{"abcdef", 6, 0, 0, "", ""},
		{"abcdef", 6, 3, 0, "", ""},
		{"abcdef", 6, 0, 6, "abcdef", ""},
		{"abcdef", 6, 0, 5, "abcde", ""},
		{"abcdef", 6, 0, 4, "abcd", ""},
		{"abcdef", 6, 1, 4, "bcde", ""},
		{"abcdef", 6, 2, 4, "cdef", ""},
		{"abcdefg", 7, 1, 4, "bcde", ""},
		{"abcdef", 6, 0, 7, "abcdef", "pieceRanger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "pieceRanger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "pieceRanger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		route := pb.NewMockPieceStoreRoutesClient(ctrl)

		calls := []*gomock.Call{
			route.EXPECT().Piece(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(&pb.PieceSummary{Size: int64(len(tt.data))}, nil),
		}

		stream := pb.NewMockPieceStoreRoutes_RetrieveClient(ctrl)
		pid := NewPieceID()

		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			calls = append(calls,
				stream.EXPECT().Send(
					&pb.PieceRetrieval{
						PieceData: &pb.PieceRetrieval_PieceData{
							Id: pid.String(), Size: tt.length, Offset: tt.offset,
						},
					},
				).Return(nil),
				stream.EXPECT().Send(
					&pb.PieceRetrieval{
						Bandwidthallocation: &pb.RenterBandwidthAllocation{
							Data: serializeData(&pb.RenterBandwidthAllocation_Data{
								PayerAllocation: &pb.PayerBandwidthAllocation{},
								Total:           32 * 1024,
							}),
						},
					},
				).Return(nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.length,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Send(
					&pb.PieceRetrieval{
						Bandwidthallocation: &pb.RenterBandwidthAllocation{
							Data: serializeData(&pb.RenterBandwidthAllocation_Data{
								PayerAllocation: &pb.PayerBandwidthAllocation{},
								Total:           32 * 1024 * 2,
							}),
						},
					},
				).Return(nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}
		gomock.InOrder(calls...)

		ctx := context.Background()
		c, err := NewCustomRoute(route, 32*1024)
		assert.NoError(t, err)
		rr, err := PieceRanger(ctx, c, stream, pid, &pb.PayerBandwidthAllocation{})
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.size, rr.Size(), errTag)
		}
		r, err := rr.Range(ctx, tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}

func TestPieceRangerSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for i, tt := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		errString            string
	}{
		{"", 0, 0, 0, "", ""},
		{"abcdef", 6, 0, 0, "", ""},
		{"abcdef", 6, 3, 0, "", ""},
		{"abcdef", 6, 0, 6, "abcdef", ""},
		{"abcdef", 6, 0, 5, "abcde", ""},
		{"abcdef", 6, 0, 4, "abcd", ""},
		{"abcdef", 6, 1, 4, "bcde", ""},
		{"abcdef", 6, 2, 4, "cdef", ""},
		{"abcdefg", 7, 1, 4, "bcde", ""},
		{"abcdef", 6, 0, 7, "abcdef", "pieceRanger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "pieceRanger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "pieceRanger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		route := pb.NewMockPieceStoreRoutesClient(ctrl)
		pid := NewPieceID()

		stream := pb.NewMockPieceStoreRoutes_RetrieveClient(ctrl)

		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			gomock.InOrder(
				stream.EXPECT().Send(
					&pb.PieceRetrieval{
						PieceData: &pb.PieceRetrieval_PieceData{
							Id: pid.String(), Size: tt.length, Offset: tt.offset,
						},
					},
				).Return(nil),
				stream.EXPECT().Send(
					&pb.PieceRetrieval{Bandwidthallocation: &pb.RenterBandwidthAllocation{
						Data: serializeData(&pb.RenterBandwidthAllocation_Data{
							PayerAllocation: &pb.PayerBandwidthAllocation{},
							Total:           32 * 1024,
						}),
					},
					},
				).Return(nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.length,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Send(
					&pb.PieceRetrieval{
						Bandwidthallocation: &pb.RenterBandwidthAllocation{
							Data: serializeData(&pb.RenterBandwidthAllocation_Data{
								PayerAllocation: &pb.PayerBandwidthAllocation{},
								Total:           32 * 1024 * 2,
							}),
						},
					},
				).Return(nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}

		ctx := context.Background()
		c, err := NewCustomRoute(route, 32*1024)
		assert.NoError(t, err)
		rr := PieceRangerSize(c, stream, pid, tt.size, &pb.PayerBandwidthAllocation{})
		assert.Equal(t, tt.size, rr.Size(), errTag)
		r, err := rr.Range(ctx, tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}

func serializeData(ba *pb.RenterBandwidthAllocation_Data) []byte {
	data, _ := proto.Marshal(ba)

	return data
}
