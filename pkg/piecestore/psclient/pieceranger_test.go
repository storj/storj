// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/storj"

	"storj.io/storj/pkg/pb"
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

		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.Nil(t, err)

		route := pb.NewMockPieceStoreRoutesClient(ctrl)

		route.EXPECT().Piece(
			gomock.Any(), gomock.Any(), gomock.Any(),
		).Return(&pb.PieceSummary{PieceSize: int64(len(tt.data))}, nil)

		stream := pb.NewMockPieceStoreRoutes_RetrieveClient(ctrl)
		pid := NewPieceID()

		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			msg1 := &pb.PieceRetrieval{
				PieceData: &pb.PieceRetrieval_PieceData{
					Id: pid.String(), PieceSize: tt.length, Offset: tt.offset,
				},
			}

			stream.EXPECT().Send(msg1).Return(nil)
			stream.EXPECT().Send(gomock.Any()).Return(nil).MinTimes(0).MaxTimes(1)
			stream.EXPECT().Recv().Return(
				&pb.PieceRetrievalStream{
					PieceSize: tt.length,
					Content:   []byte(tt.data)[tt.offset : tt.offset+tt.length],
				}, nil)
			stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF)
		}

		ctx := context.Background()

		target := &pb.Node{
			Address: &pb.NodeAddress{
				Address:   "",
				Transport: 0,
			},
			Id: teststorj.NodeIDFromString("test-node-id-1234567"),
		}
		c, err := NewCustomRoute(route, target, 32*1024, priv)
		assert.NoError(t, err)
		rr, err := PieceRanger(ctx, c, stream, pid, &pb.PayerBandwidthAllocation{}, nil)
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

		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		assert.Nil(t, err)

		if tt.offset >= 0 && tt.length > 0 && tt.offset+tt.length <= tt.size {
			msg1 := &pb.PieceRetrieval{
				PieceData: &pb.PieceRetrieval_PieceData{
					Id: pid.String(), PieceSize: tt.length, Offset: tt.offset,
				},
			}

			stream.EXPECT().Send(msg1).Return(nil)
			stream.EXPECT().Send(gomock.Any()).Return(nil).MinTimes(0).MaxTimes(1)
			stream.EXPECT().Recv().Return(
				&pb.PieceRetrievalStream{
					PieceSize: tt.length,
					Content:   []byte(tt.data)[tt.offset : tt.offset+tt.length],
				}, nil)
			stream.EXPECT().Recv().Return(&pb.PieceRetrievalStream{}, io.EOF)
		}

		ctx := context.Background()

		target := &pb.Node{
			Address: &pb.NodeAddress{
				Address:   "",
				Transport: 0,
			},
			Id: teststorj.NodeIDFromString("test-node-id-1234567"),
		}
		c, err := NewCustomRoute(route, target, 32*1024, priv)
		assert.NoError(t, err)
		rr := PieceRangerSize(c, stream, pid, tt.size, &pb.PayerBandwidthAllocation{}, nil)
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
