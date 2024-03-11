// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/bloomfilter"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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

	req := retain.Request{
		SatelliteID:   testrand.NodeID(),
		CreatedBefore: time.Now(),
		Filter:        filter,
	}

	err := retain.SaveRequest(retainDir, req)
	require.NoError(t, err)

	store, err := retain.NewRequestStore(retainDir)
	require.NoError(t, err)
	require.Equal(t, 1, store.Len())

	actualData := store.Data()[req.SatelliteID]

	require.Equal(t, req.CreatedBefore.UTC(), actualData.CreatedBefore.UTC())
	require.Equal(t, req.Filter, actualData.Filter)
}

func TestNewRequestStore_invalidFilenames(t *testing.T) {
	ctx := testcontext.New(t)

	retainDir := ctx.Dir("retain")

	filter := bloomfilter.NewOptimal(5, 0.000000001)
	pieceIDs := generateTestIDs(5)

	for _, pieceID := range pieceIDs {
		filter.Add(pieceID)
	}

	req := retain.Request{
		SatelliteID:   testrand.NodeID(),
		CreatedBefore: time.Now(),
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
	err := retain.SaveRequest(retainDir, req)
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
