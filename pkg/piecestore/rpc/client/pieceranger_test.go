// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

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
				stream.EXPECT().Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: pid.String(), Size: tt.length, Offset: tt.offset}}).Return(nil),
				stream.EXPECT().Send(&pb.PieceRetrieval{Bandwidthallocation: &pb.BandwidthAllocation{Data: &pb.BandwidthAllocation_Data{Payer: "payer-id", Renter: "renter-id", Size: 32768}}}).Return(nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.length,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Send(&pb.PieceRetrieval{Bandwidthallocation: &pb.BandwidthAllocation{Data: &pb.BandwidthAllocation_Data{Payer: "payer-id", Renter: "renter-id", Size: 32768}}}).Return(nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}
		gomock.InOrder(calls...)

		ctx := context.Background()
		c := NewCustomRoute(route, "payer-id", "renter-id")
		rr, err := PieceRanger(ctx, c, stream, pid)
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
				stream.EXPECT().Send(&pb.PieceRetrieval{PieceData: &pb.PieceRetrieval_PieceData{Id: pid.String(), Size: tt.length, Offset: tt.offset}}).Return(nil),
				stream.EXPECT().Send(&pb.PieceRetrieval{Bandwidthallocation: &pb.BandwidthAllocation{Data: &pb.BandwidthAllocation_Data{Payer: "payer-id", Renter: "renter-id", Size: 32768}}}).Return(nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.length,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Send(&pb.PieceRetrieval{Bandwidthallocation: &pb.BandwidthAllocation{Data: &pb.BandwidthAllocation_Data{Payer: "payer-id", Renter: "renter-id", Size: 32768}}}).Return(nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}

		ctx := context.Background()
		c := NewCustomRoute(route, "payer-id", "renter-id")
		rr := PieceRangerSize(c, stream, pid, tt.size)
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
