// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
)

func Test_unmarshalJSONNodes(t *testing.T) {
	nodeID, err := storj.NodeIDFromString("1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR")
	require.NoError(t, err)

	t.Run("valid json object", func(t *testing.T) {
		nodesJSONData := `
{
	"name": "Storagenode 1",
	"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
	"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
	"apiSecret": "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI="
}
`
		expectedNodeInfo := []nodeInfo{
			{
				NodeID:        nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI=",
				Name:          "Storagenode 1",
			},
		}

		got, err := unmarshalJSONNodes([]byte(nodesJSONData))
		require.NoError(t, err)

		require.Equal(t, expectedNodeInfo, got)
	})

	t.Run("valid json array", func(t *testing.T) {
		nodesJSONData := `
[
	{
		"name": "Storagenode 1",
		"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
		"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
		"apiSecret": "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI="
	}
]
`
		expectedNodeInfo := []nodeInfo{
			{
				NodeID:        nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI=",
				Name:          "Storagenode 1",
			},
		}

		got, err := unmarshalJSONNodes([]byte(nodesJSONData))
		require.NoError(t, err)

		require.Equal(t, expectedNodeInfo, got)
	})
}
