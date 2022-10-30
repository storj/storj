// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.
package blockchain_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/blockchain"
)

func TestBytesToAddress(t *testing.T) {
	a := blockchain.Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 10}
	gotA, err := blockchain.BytesToAddress(a.Bytes())
	require.NoError(t, err)
	require.Equal(t, a, gotA)

	_, err = blockchain.BytesToAddress([]byte{1, 2, 3})
	require.Error(t, err)
}

func TestAddressHex(t *testing.T) {
	addresses := []string{
		"0xDAFEA492D9c6733ae3d56b7Ed1ADB60692c98Bc5",
		"0xd24400ae8BfEBb18cA49Be86258a3C749cf46853",
		"0x4E58657CD8b3401B6f7Db9Cd0408f06582c458b5",
	}
	for _, address := range addresses {
		decoded, err := hex.DecodeString(address[2:])
		require.NoError(t, err)

		a, err := blockchain.BytesToAddress(decoded)
		require.NoError(t, err)

		require.Equal(t, address, a.Hex())
	}
}

func TestHashHex(t *testing.T) {
	hashes := []string{
		"0xdea1082dbea119c822dfe804264f5b880d4208ef51e8c5a8995eff10a5094de8",
		"0xd1a78f16158b550945dd39182d188c3fb7285b431223c8b0fe38f4181e9ac197",
		"0x47a5b3fae0bad45631a65324001f75dea311898e26ee10597a66953b57dfe332",
	}
	for _, hash := range hashes {
		decoded, err := hex.DecodeString(hash[2:])
		require.NoError(t, err)

		h, err := blockchain.BytesToHash(decoded)
		require.NoError(t, err)

		require.Equal(t, hash, h.Hex())
	}
}
