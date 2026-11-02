// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"runtime"

	"golang.org/x/sys/cpu"

	"storj.io/common/pb"
)

// noiseProto returns the Noise protocol the server advertises.
//
// "auto" picks AES-GCM when the CPU has hardware support for it, and
// ChaCha20-Poly1305 otherwise. With hardware support AES-GCM is ~30-45%
// faster; without it, Go's AES-GCM is ~13x slower than ChaCha20-Poly1305.
func noiseProto(cipher string) (pb.NoiseProtocol, error) {
	switch cipher {
	case "", "auto":
		if hasAESGCMHardware() {
			return pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B, nil
		}
		return pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B, nil
	case "chacha":
		return pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B, nil
	case "aes":
		return pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B, nil
	default:
		return pb.NoiseProtocol_NOISE_UNSET, Error.New("unknown noise cipher %q, expected auto, chacha or aes", cipher)
	}
}

// hasAESGCMHardware reports whether Go's AES-GCM uses the assembly
// implementation on this CPU. It uses the same checks as the Go standard
// library (crypto/internal/fips140/aes/gcm), limited to amd64 and arm64.
func hasAESGCMHardware() bool {
	switch runtime.GOARCH {
	case "amd64":
		return cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ && cpu.X86.HasSSE41 && cpu.X86.HasSSSE3
	case "arm64":
		return cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
	default:
		return false
	}
}
