// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"github.com/flynn/noise"

	"storj.io/common/pb"
)

// NoiseHeader is the drpcmigrate.Header prefix for DRPC over Noise.
const NoiseHeader = "DRPC!N!1"

// defaultNoiseProto is the protobuf enum value that specifies what noise
// protocol is in use.
// defaultNoiseInfo and defaultNoiseConfig should be changed together.
var defaultNoiseProto = pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B

// defaultNoiseConfig returns the structure that tells this node what Noise
// settings to use.
// defaultNoiseProto and defaultNoiseConfig should be changed together.
func defaultNoiseConfig() noise.Config {
	return noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b),
		Pattern:     noise.HandshakeIK,
	}
}
