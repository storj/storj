// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestNoiseKeyAttestation(t *testing.T) {
	ctx := testcontext.New(t)
	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)
	ident2, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)
	ident3, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)

	noiseCfg, err := generateNoiseConf(ident)
	require.NoError(t, err)

	attestation, err := GenerateNoiseKeyAttestation(ctx, ident, &pb.NoiseInfo{
		Proto:     defaultNoiseProto,
		PublicKey: noiseCfg.StaticKeypair.Public,
	})
	require.NoError(t, err)

	require.NoError(t, ValidateNoiseKeyAttestation(ctx, attestation))

	badAttestation1 := *attestation
	badAttestation1.NodeId, err = storj.NodeIDFromString("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6")
	require.NoError(t, err)
	err = ValidateNoiseKeyAttestation(ctx, &badAttestation1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "node id mismatch")

	badAttestation2 := *attestation
	badAttestation2.NoisePublicKey = badAttestation2.NoisePublicKey[:len(badAttestation2.NoisePublicKey)-1]
	err = ValidateNoiseKeyAttestation(ctx, &badAttestation2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature is not valid")

	badAttestation3 := *attestation
	badAttestation3.Timestamp = time.Now()
	err = ValidateNoiseKeyAttestation(ctx, &badAttestation3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature is not valid")

	ident2.CA = ident.CA
	badAttestation4 := *attestation
	badAttestation4.NodeCertchain = identity.EncodePeerIdentity(ident2.PeerIdentity())
	err = ValidateNoiseKeyAttestation(ctx, &badAttestation4)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate chain invalid")

	ident3.Leaf = ident.Leaf
	badAttestation5 := *attestation
	badAttestation5.NodeCertchain = identity.EncodePeerIdentity(ident3.PeerIdentity())
	err = ValidateNoiseKeyAttestation(ctx, &badAttestation5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate chain invalid")
}

func TestNoiseSessionAttestation(t *testing.T) {
	ctx := testcontext.New(t)
	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)
	ident2, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)
	ident3, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{})
	require.NoError(t, err)

	var hash [32]byte
	_, err = rand.Read(hash[:])
	require.NoError(t, err)

	attestation, err := GenerateNoiseSessionAttestation(ctx, ident, hash[:])
	require.NoError(t, err)

	require.NoError(t, ValidateNoiseSessionAttestation(ctx, attestation))

	badAttestation1 := *attestation
	badAttestation1.NodeId, err = storj.NodeIDFromString("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6")
	require.NoError(t, err)
	err = ValidateNoiseSessionAttestation(ctx, &badAttestation1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "node id mismatch")

	badAttestation2 := *attestation
	badAttestation2.NoiseHandshakeHash = badAttestation2.NoiseHandshakeHash[:len(badAttestation2.NoiseHandshakeHash)-1]
	err = ValidateNoiseSessionAttestation(ctx, &badAttestation2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature is not valid")

	ident2.CA = ident.CA
	badAttestation3 := *attestation
	badAttestation3.NodeCertchain = identity.EncodePeerIdentity(ident2.PeerIdentity())
	err = ValidateNoiseSessionAttestation(ctx, &badAttestation3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate chain invalid")

	ident3.Leaf = ident.Leaf
	badAttestation4 := *attestation
	badAttestation4.NodeCertchain = identity.EncodePeerIdentity(ident3.PeerIdentity())
	err = ValidateNoiseSessionAttestation(ctx, &badAttestation4)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate chain invalid")
}
