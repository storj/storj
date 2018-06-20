// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/piecestore/rpc/client"
	pb "storj.io/storj/protos/piecestore"
)

func TestGRPCRanger(t *testing.T) {
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
		{"abcdef", 6, 0, 7, "abcdef", "ranger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "ranger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "ranger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		route := pb.NewMockPieceStoreRoutesClient(ctrl)
		calls := []*gomock.Call{
			route.EXPECT().Piece(
				gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(&pb.PieceSummary{Size: int64(len(tt.data))}, nil),
		}
		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			stream := pb.NewMockPieceStoreRoutes_RetrieveClient(ctrl)
			calls = append(calls,
				route.EXPECT().Retrieve(
					gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(stream, nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.length,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}
		gomock.InOrder(calls...)

		ctx := context.Background()
		c := client.NewCustomRoute(route)
		rr, err := GRPCRanger(ctx, c, "")
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.size, rr.Size(), errTag)
		}
		r, err := rr.Range(ctx, tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			return
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}

func TestGRPCRangerSize(t *testing.T) {
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
		{"abcdef", 6, 0, 7, "abcdef", "ranger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "ranger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "ranger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		route := pb.NewMockPieceStoreRoutesClient(ctrl)
		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			stream := pb.NewMockPieceStoreRoutes_RetrieveClient(ctrl)
			gomock.InOrder(
				route.EXPECT().Retrieve(
					gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(stream, nil),
				stream.EXPECT().Recv().Return(
					&pb.PieceRetrievalStream{
						Size:    tt.size,
						Content: []byte(tt.data)[tt.offset : tt.offset+tt.length],
					}, nil),
				stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF),
			)
		}

		ctx := context.Background()
		c := client.NewCustomRoute(route)
		rr := GRPCRangerSize(c, "", tt.size)
		assert.Equal(t, tt.size, rr.Size(), errTag)
		r, err := rr.Range(ctx, tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			return
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}
