// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/subtle"
	"encoding/binary"
	"time"

	"github.com/flynn/noise"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
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

func signableNoisePublicKey(ts time.Time, key []byte) []byte {
	var buf [8]byte
	tsnano := ts.UnixNano()
	if tsnano < 0 {
		tsnano = 0
	}
	binary.BigEndian.PutUint64(buf[:], uint64(tsnano))
	return append(buf[:], key...)
}

// GenerateNoiseKeyAttestation will sign a given Noise public key using the
// Node's leaf key and certificate chain, generating a pb.NoiseKeyAttestation.
func GenerateNoiseKeyAttestation(ctx context.Context, ident *identity.FullIdentity, info *pb.NoiseInfo) (_ *pb.NoiseKeyAttestation, err error) {
	defer mon.Task()(&ctx)(&err)
	ts := time.Now()
	signature, err := signing.SignerFromFullIdentity(ident).HashAndSign(ctx,
		append([]byte("noise-key-attestation-v1:"), signableNoisePublicKey(ts, info.PublicKey)...))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.NoiseKeyAttestation{
		NodeId:         ident.ID,
		NodeCertchain:  identity.EncodePeerIdentity(ident.PeerIdentity()),
		NoiseProto:     info.Proto,
		NoisePublicKey: info.PublicKey,
		Timestamp:      ts,
		Signature:      signature,
	}, nil
}

// ValidateNoiseKeyAttestation will confirm that a provided
// *pb.NoiseKeyAttestation was signed correctly.
func ValidateNoiseKeyAttestation(ctx context.Context, attestation *pb.NoiseKeyAttestation) (err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := identity.DecodePeerIdentity(ctx, attestation.NodeCertchain)
	if err != nil {
		return Error.Wrap(err)
	}
	err = peer.Leaf.CheckSignatureFrom(peer.CA)
	if err != nil {
		return Error.New("certificate chain invalid: %w", err)
	}

	if subtle.ConstantTimeCompare(peer.ID.Bytes(), attestation.NodeId.Bytes()) != 1 {
		return Error.New("node id mismatch")
	}
	signee := signing.SigneeFromPeerIdentity(peer)
	unsigned := signableNoisePublicKey(attestation.Timestamp, attestation.NoisePublicKey)
	err = signee.HashAndVerifySignature(ctx,
		append([]byte("noise-key-attestation-v1:"), unsigned...),
		attestation.Signature)
	return Error.Wrap(err)
}

// GenerateNoiseSessionAttestation will sign a given Noise session handshake
// hash using the Node's leaf key and certificate chain, generating a
// pb.NoiseSessionAttestation.
func GenerateNoiseSessionAttestation(ctx context.Context, ident *identity.FullIdentity, handshakeHash []byte) (_ *pb.NoiseSessionAttestation, err error) {
	defer mon.Task()(&ctx)(&err)
	signature, err := signing.SignerFromFullIdentity(ident).HashAndSign(ctx,
		append([]byte("noise-session-attestation-v1:"), handshakeHash...))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.NoiseSessionAttestation{
		NodeId:             ident.ID,
		NodeCertchain:      identity.EncodePeerIdentity(ident.PeerIdentity()),
		NoiseHandshakeHash: handshakeHash,
		Signature:          signature,
	}, nil
}

// ValidateNoiseSessionAttestation will confirm that a provided
// *pb.NoiseSessionAttestation was signed correctly.
func ValidateNoiseSessionAttestation(ctx context.Context, attestation *pb.NoiseSessionAttestation) (err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := identity.DecodePeerIdentity(ctx, attestation.NodeCertchain)
	if err != nil {
		return Error.Wrap(err)
	}
	err = peer.Leaf.CheckSignatureFrom(peer.CA)
	if err != nil {
		return Error.New("certificate chain invalid: %w", err)
	}

	if subtle.ConstantTimeCompare(peer.ID.Bytes(), attestation.NodeId.Bytes()) != 1 {
		return Error.New("node id mismatch")
	}
	signee := signing.SigneeFromPeerIdentity(peer)
	err = signee.HashAndVerifySignature(ctx,
		append([]byte("noise-session-attestation-v1:"), attestation.NoiseHandshakeHash...),
		attestation.Signature)
	return Error.Wrap(err)

}
