// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/retain"
)

func TestNewRequestStore(t *testing.T) {
	ctx := testcontext.New(t)

	retainDir := ctx.Dir("retain")

	filter := bloomfilter.NewOptimal(5, 0.000000001)
	pieceIDs := generateTestIDs(5)

	for _, pieceID := range pieceIDs {
		filter.Add(pieceID)
	}

	hasher := pb.NewHashFromAlgorithm(pb.PieceHashAlgorithm_BLAKE3)
	_, err := hasher.Write(filter.Bytes())
	require.NoError(t, err)

	pbReq := &pb.RetainRequest{
		CreationDate:  time.Now().UTC(),
		Filter:        filter.Bytes(),
		HashAlgorithm: pb.PieceHashAlgorithm_BLAKE3,
		Hash:          hasher.Sum(nil),
	}

	req := retain.Request{
		SatelliteID:   testrand.NodeID(),
		CreatedBefore: pbReq.CreationDate,
		Filter:        filter,
	}

	err = retain.SaveRequest(retainDir, req.GetFilename(), pbReq)
	require.NoError(t, err)

	store, err := retain.NewRequestStore(retainDir)
	require.NoError(t, err)
	require.Equal(t, 1, store.Len())

	actualData := store.Data()[req.SatelliteID]

	require.Equal(t, req.CreatedBefore.UTC(), actualData.CreatedBefore.UTC())
	require.Equal(t, req.Filter, actualData.Filter)

	// simulate newer request
	req.CreatedBefore = time.Now().UTC()
	pbReq.CreationDate = req.CreatedBefore

	// replace existing entry
	added, err := store.Add(req.SatelliteID, pbReq)
	require.NoError(t, err)
	require.True(t, added)

	actualData = store.Data()[req.SatelliteID]
	require.Equal(t, req.CreatedBefore.UTC(), actualData.CreatedBefore.UTC())
	require.Equal(t, req.Filter, actualData.Filter)

	files, err := os.ReadDir(retainDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	for _, file := range files {
		require.NoError(t, os.Remove(filepath.Join(retainDir, file.Name())))
	}

	// simulate newer request
	req.CreatedBefore = time.Now()
	pbReq.CreationDate = req.CreatedBefore

	// replace existing entry without file on disk
	added, err = store.Add(req.SatelliteID, pbReq)
	require.NoError(t, err)
	require.True(t, added)

	actualData = store.Data()[req.SatelliteID]
	require.Equal(t, req.CreatedBefore.UTC(), actualData.CreatedBefore.UTC())
	require.Equal(t, req.Filter, actualData.Filter)
}

func TestRequestStore_truncated(t *testing.T) {
	ctx := testcontext.New(t)

	retainDir := ctx.Dir("retain")

	filter := bloomfilter.NewOptimal(10000000, 0.000000001)

	for _, pieceID := range generateTestIDs(1000) {
		filter.Add(pieceID)
	}

	hasher := pb.NewHashFromAlgorithm(pb.PieceHashAlgorithm_BLAKE3)
	_, err := hasher.Write(filter.Bytes())
	require.NoError(t, err)

	pbReq := &pb.RetainRequest{
		CreationDate:  time.Now(),
		Filter:        filter.Bytes(),
		HashAlgorithm: pb.PieceHashAlgorithm_BLAKE3,
		Hash:          hasher.Sum(nil),
	}

	req := retain.Request{
		SatelliteID:   testrand.NodeID(),
		CreatedBefore: pbReq.GetCreationDate(),
		Filter:        filter,
	}

	err = retain.SaveRequest(retainDir, req.GetFilename(), pbReq)
	require.NoError(t, err)

	{
		// here we truncate the file, simulating a partial disk write
		files, err := os.ReadDir(retainDir)
		require.NoError(t, err)
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			info, err := f.Info()
			require.NoError(t, err)
			err = os.Truncate(filepath.Join(retainDir, f.Name()), info.Size()-10)
			require.NoError(t, err)
		}
	}

	store, err := retain.NewRequestStore(retainDir)
	require.Error(t, err) // most likely to fail at pb.Unmarshal
	t.Log(err)
	require.Equal(t, 0, store.Len())
}

func TestNewRequestStore_invalidFilenames(t *testing.T) {
	ctx := testcontext.New(t)

	retainDir := ctx.Dir("retain")

	filter := bloomfilter.NewOptimal(5, 0.000000001)
	pieceIDs := generateTestIDs(5)

	for _, pieceID := range pieceIDs {
		filter.Add(pieceID)
	}

	pbReq := &pb.RetainRequest{
		CreationDate: time.Now(),
		Filter:       filter.Bytes(),
	}

	req := retain.Request{
		SatelliteID:   testrand.NodeID(),
		CreatedBefore: pbReq.CreationDate,
		Filter:        filter,
	}

	type testData struct {
		filename string
		bytes    []byte
		error    string
	}

	files := []testData{
		{
			filename: "ignoreme",
			bytes:    []byte("ignoreme"),
			error:    "invalid filename: ignoreme;",
		},
		{
			filename: "research-paper.pdf",
			bytes:    []byte("%PDF"),
			error:    "invalid filename: research-paper.pdf;",
		},
		{
			filename: "bi-monthly.txt",
			bytes:    []byte("data"),
			error:    "invalid filename: bi-monthly.txt;",
		},
		{
			filename: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbb.range",
			bytes:    []byte("data"),
			error:    "invalid filename: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbb.range;",
		},
		{
			// valid base32 encoded node id, but invalid unix time
			filename: "d3n5vhlqfaicvtyino5d6oqrp5ws6qsjnieninn76kci32xkqyaa-aaaaaaa",
			bytes:    []byte("data"),
			error:    "invalid filename: d3n5vhlqfaicvtyino5d6oqrp5ws6qsjnieninn76kci32xkqyaa-aaaaaaa;",
		},
		{
			// valid base32 encoded node id and unix time, but invalid filter
			filename: "d3n5vhlqfaicvtyino5d6oqrp5ws6qsjnieninn76kci32xkqyaa-1234567890",
			bytes:    []byte("data"),
			error:    "malformed bloom filter",
		},
		{
			// valid base32 encoded node id, and invalid time
			filename: "d3n5vhlqfaicvtyino5d6oqrp5ws6qsjnieninn76kci32xkqyaa-0",
			bytes:    []byte("data"),
			error:    "invalid filename: d3n5vhlqfaicvtyino5d6oqrp5ws6qsjnieninn76kci32xkqyaa-0; failed time validation",
		},
	}

	// create some files that should be ignored
	for _, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(retainDir, data.filename), data.bytes, 0644))
	}

	// create a valid file
	err := retain.SaveRequest(retainDir, req.GetFilename(), pbReq)
	require.NoError(t, err)

	store, err := retain.NewRequestStore(retainDir)
	require.Error(t, err)
	for _, data := range files {
		require.Contains(t, err.Error(), data.error)
	}
	t.Log(err)
	require.Equal(t, 1, store.Len())

	actualData := store.Data()[req.SatelliteID]

	require.Equal(t, req.CreatedBefore.UTC(), actualData.CreatedBefore.UTC())
	require.Equal(t, req.Filter, actualData.Filter)
}
