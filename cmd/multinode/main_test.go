// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"

	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodeauth"
)

func TestUnmarshalJSONNodes(t *testing.T) {
	nodeID, err := storj.NodeIDFromString("1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR")
	require.NoError(t, err)

	apiSecret, err := multinodeauth.SecretFromBase64("b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI=")
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
		expectedNodeInfo := []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     apiSecret,
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
		expectedNodeInfo := []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     apiSecret,
				Name:          "Storagenode 1",
			},
		}

		got, err := unmarshalJSONNodes([]byte(nodesJSONData))
		require.NoError(t, err)

		require.Equal(t, expectedNodeInfo, got)
	})

	t.Run("invalid base64 input, expects base64url", func(t *testing.T) {
		nodesJSONData := `
{
	"name": "Storagenode 1",
	"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
	"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
	"apiSecret": "b/yeI0OBKBusBVN4/dHxpxlwdTyoFPwtEuHv9ACl9jI="
}
`
		got, err := unmarshalJSONNodes([]byte(nodesJSONData))
		require.Error(t, err)
		require.ErrorIs(t, err, base64.CorruptInputError(1))
		require.Nil(t, got)
	})

	t.Run("invalid secret", func(t *testing.T) {
		nodesJSONData := `
{
	"name": "Storagenode 1",
	"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
	"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
	"apiSecret": "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI-"
}
`
		got, err := unmarshalJSONNodes([]byte(nodesJSONData))
		require.Error(t, err)
		require.Equal(t, "invalid secret", err.Error())
		require.Nil(t, got)
	})

	t.Run("JSON with UTF8 byte order mark sequence", func(t *testing.T) {
		nodesJSONData := "\xef\xbb\xbf{\"name\": \"Storagenode 1\",\"id\":\"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR\",\"publicAddress\": \"awn7k09ts6mxbgau.myfritz.net:13010\",\"apiSecret\": \"b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI=\"}"
		got, err := unmarshalJSONNodes([]byte(nodesJSONData))

		expectedNodeInfo := []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     apiSecret,
				Name:          "Storagenode 1",
			},
		}

		require.NoError(t, err)
		require.Equal(t, expectedNodeInfo, got)
	})

	t.Run("UTF-16LE byte order mark sequence", func(t *testing.T) {
		nodesJSONData := `{
		"name": "Storagenode 1",
		"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
		"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
		"apiSecret": "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI="
	}`
		// encode to UTF-16LE
		enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder()
		utf16LEStr, err := enc.String(nodesJSONData)
		require.NoError(t, err)

		expectedNodeInfo := []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     apiSecret,
				Name:          "Storagenode 1",
			},
		}

		got, err := unmarshalJSONNodes([]byte(utf16LEStr))
		require.NoError(t, err)

		require.Equal(t, expectedNodeInfo, got)
	})

	t.Run("UTF-16BE byte order mark sequence", func(t *testing.T) {
		nodesJSONData := `{
		"name": "Storagenode 1",
		"id":"1MJ7R1cqGrFnELPY3YKd62TBJ6vE8x9yPKPwUFHUx6G8oypezR",
		"publicAddress": "awn7k09ts6mxbgau.myfritz.net:13010",
		"apiSecret": "b_yeI0OBKBusBVN4_dHxpxlwdTyoFPwtEuHv9ACl9jI="
	}`
		// encode to UTF-16BE
		enc := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder()
		utf16BEStr, err := enc.String(nodesJSONData)
		require.NoError(t, err)

		expectedNodeInfo := []nodes.Node{
			{
				ID:            nodeID,
				PublicAddress: "awn7k09ts6mxbgau.myfritz.net:13010",
				APISecret:     apiSecret,
				Name:          "Storagenode 1",
			},
		}

		got, err := unmarshalJSONNodes([]byte(utf16BEStr))
		require.NoError(t, err)

		require.Equal(t, expectedNodeInfo, got)
	})
}
