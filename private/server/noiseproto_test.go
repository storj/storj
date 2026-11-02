// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
)

func TestNoiseProto(t *testing.T) {
	auto := pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B
	if hasAESGCMHardware() {
		auto = pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B
	}

	for cipher, expected := range map[string]pb.NoiseProtocol{
		"":       auto,
		"auto":   auto,
		"aes":    pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B,
		"chacha": pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B,
	} {
		proto, err := noiseProto(cipher)
		require.NoError(t, err, cipher)
		require.Equal(t, expected, proto, cipher)
	}

	_, err := noiseProto("des")
	require.Error(t, err)
}
