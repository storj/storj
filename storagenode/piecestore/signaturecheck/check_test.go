// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package signaturecheck_test

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/piecestore/signaturecheck"
)

func TestFull_VerifyUplinkOrderSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	full := &signaturecheck.Full{}

	// Create test piece keys
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	// Create a test order
	order := &pb.Order{
		SerialNumber: testrand.SerialNumber(),
		Amount:       1000,
	}

	t.Run("valid signature", func(t *testing.T) {
		// Sign the order
		signature, err := signing.SignUplinkOrder(ctx, piecePrivateKey, order)
		order.UplinkSignature = signature.GetUplinkSignature()
		require.NoError(t, err)

		// Verify the signature
		err = full.VerifyUplinkOrderSignature(ctx, piecePublicKey, order)
		assert.NoError(t, err)
	})

	t.Run("invalid signature", func(t *testing.T) {
		// Create order with invalid signature
		invalidOrder := &pb.Order{
			SerialNumber:    testrand.SerialNumber(),
			Amount:          2000,
			UplinkSignature: []byte("invalid signature"),
		}

		// Verify should fail
		err = full.VerifyUplinkOrderSignature(ctx, piecePublicKey, invalidOrder)
		assert.Error(t, err)
	})

	t.Run("wrong public key", func(t *testing.T) {
		// Create different piece keys
		wrongPublicKey, _, err := storj.NewPieceKey()
		require.NoError(t, err)

		// Sign with original key
		_, err = signing.SignUplinkOrder(ctx, piecePrivateKey, order)
		require.NoError(t, err)

		// Verify with wrong key should fail
		err = full.VerifyUplinkOrderSignature(ctx, wrongPublicKey, order)
		assert.Error(t, err)
	})
}

func TestFull_VerifyOrderLimitSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	full := &signaturecheck.Full{}

	// Create satellite identity
	satelliteIdentity, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	require.NoError(t, err)

	// Create order limit
	piecePublicKey, _, err := storj.NewPieceKey()
	require.NoError(t, err)

	orderLimit := &pb.OrderLimit{
		SatelliteId:     satelliteIdentity.ID,
		UplinkPublicKey: piecePublicKey,
		StorageNodeId:   testrand.NodeID(),
		PieceId:         testrand.PieceID(),
		Action:          pb.PieceAction_PUT,
		SerialNumber:    testrand.SerialNumber(),
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(time.Hour),
		PieceExpiration: time.Now().Add(24 * time.Hour),
		Limit:           1000,
	}

	t.Run("valid signature", func(t *testing.T) {
		satelliteIdentity := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())

		// Sign the order limit
		signature, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satelliteIdentity), orderLimit)
		require.NoError(t, err)
		orderLimit.SatelliteSignature = signature.GetSatelliteSignature()

		// Verify the signature
		err = full.VerifyOrderLimitSignature(ctx, signing.SignerFromFullIdentity(satelliteIdentity), orderLimit)
		assert.NoError(t, err)
	})

	t.Run("invalid signature", func(t *testing.T) {
		// Create order limit with invalid signature
		invalidOrderLimit := &pb.OrderLimit{
			SatelliteId:        satelliteIdentity.ID,
			UplinkPublicKey:    piecePublicKey,
			StorageNodeId:      testrand.NodeID(),
			PieceId:            testrand.PieceID(),
			Action:             pb.PieceAction_PUT,
			SerialNumber:       testrand.SerialNumber(),
			OrderCreation:      time.Now(),
			OrderExpiration:    time.Now().Add(time.Hour),
			PieceExpiration:    time.Now().Add(24 * time.Hour),
			Limit:              1000,
			SatelliteSignature: []byte("invalid signature"),
		}

		// Verify should fail
		err = full.VerifyOrderLimitSignature(ctx, signing.SignerFromFullIdentity(satelliteIdentity), invalidOrderLimit)
		assert.Error(t, err)
	})

	t.Run("wrong satellite", func(t *testing.T) {
		// Create different satellite identity
		wrongSatellite, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
			Difficulty:  0,
			Concurrency: 1,
		})
		require.NoError(t, err)

		// Sign with original satellite
		signature, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satelliteIdentity), orderLimit)
		require.NoError(t, err)
		orderLimit.SatelliteSignature = signature.GetSatelliteSignature()

		// Verify with wrong satellite should fail
		err = full.VerifyOrderLimitSignature(ctx, signing.SignerFromFullIdentity(wrongSatellite), orderLimit)
		assert.Error(t, err)
	})
}

func TestTrusted_VerifyUplinkOrderSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trustedIdentity := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())
	untrustedIdentity := testidentity.MustPregeneratedIdentity(2, storj.LatestIDVersion())

	config := signaturecheck.Config{
		TrustedUplinks: []string{trustedIdentity.ID.String()},
	}

	trusted, err := signaturecheck.NewTrusted(config)
	require.NoError(t, err)

	// Create test data
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	order := &pb.Order{
		SerialNumber: testrand.SerialNumber(),
		Amount:       1000,
	}

	t.Run("trusted uplink bypasses signature check", func(t *testing.T) {

		trustedCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: trustedIdentity.Chain(),
			},
		})

		// Create order with invalid signature
		invalidOrder := &pb.Order{
			SerialNumber:    testrand.SerialNumber(),
			Amount:          2000,
			UplinkSignature: []byte("invalid signature"),
		}

		// Should pass because the peer is trusted
		err = trusted.VerifyUplinkOrderSignature(trustedCtx, piecePublicKey, invalidOrder)
		assert.NoError(t, err)
	})

	t.Run("untrusted uplink requires valid signature", func(t *testing.T) {

		ctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: untrustedIdentity.Chain(),
			},
		})

		// Sign the order properly
		signature, err := signing.SignUplinkOrder(ctx, piecePrivateKey, order)
		require.NoError(t, err)
		order.UplinkSignature = signature.UplinkSignature

		err = trusted.VerifyUplinkOrderSignature(ctx, piecePublicKey, order)
		assert.NoError(t, err)
	})

	t.Run("untrusted uplink with invalid signature fails", func(t *testing.T) {
		ctx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: untrustedIdentity.Chain(),
			},
		})

		// Create order with invalid signature
		invalidOrder := &pb.Order{
			SerialNumber:    testrand.SerialNumber(),
			Amount:          2000,
			UplinkSignature: []byte("invalid signature"),
		}

		// Should fail because signature is invalid and peer is not trusted
		err = trusted.VerifyUplinkOrderSignature(ctx, piecePublicKey, invalidOrder)
		assert.Error(t, err)
	})

	t.Run("no peer identity in context", func(t *testing.T) {
		// Use context without peer identity
		err = trusted.VerifyUplinkOrderSignature(ctx, piecePublicKey, order)
		assert.Error(t, err)
	})
}

func TestTrusted_VerifyOrderLimitSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trustedIdentity := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())

	config := signaturecheck.Config{
		TrustedUplinks: []string{trustedIdentity.ID.String()},
	}

	trusted, err := signaturecheck.NewTrusted(config)
	require.NoError(t, err)

	// Create satellite identity
	satelliteIdentity := testidentity.MustPregeneratedIdentity(3, storj.LatestIDVersion())

	// Create order limit
	piecePublicKey, _, err := storj.NewPieceKey()
	require.NoError(t, err)

	orderLimit := &pb.OrderLimit{
		SatelliteId:     satelliteIdentity.ID,
		UplinkPublicKey: piecePublicKey,
		StorageNodeId:   testrand.NodeID(),
		PieceId:         testrand.PieceID(),
		Action:          pb.PieceAction_PUT,
		SerialNumber:    testrand.SerialNumber(),
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(time.Hour),
		PieceExpiration: time.Now().Add(24 * time.Hour),
		Limit:           1000,
	}

	t.Run("trusted uplink bypasses signature check", func(t *testing.T) {

		trustedCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: trustedIdentity.Chain(),
			},
		})

		// Create order limit with invalid signature
		invalidOrderLimit := &pb.OrderLimit{
			SatelliteId:        satelliteIdentity.ID,
			UplinkPublicKey:    piecePublicKey,
			StorageNodeId:      testrand.NodeID(),
			PieceId:            testrand.PieceID(),
			Action:             pb.PieceAction_PUT,
			SerialNumber:       testrand.SerialNumber(),
			OrderCreation:      time.Now(),
			OrderExpiration:    time.Now().Add(time.Hour),
			PieceExpiration:    time.Now().Add(24 * time.Hour),
			Limit:              1000,
			SatelliteSignature: []byte("invalid signature"),
		}

		// Should pass because the peer is trusted
		err = trusted.VerifyOrderLimitSignature(trustedCtx, signing.SigneeFromPeerIdentity(satelliteIdentity.PeerIdentity()), invalidOrderLimit)
		assert.NoError(t, err)
	})

	t.Run("untrusted satellite requires valid signature", func(t *testing.T) {

		untrustedCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
			State: tls.ConnectionState{
				PeerCertificates: trustedIdentity.Chain(),
			},
		})

		invalidOrderLimit := &pb.OrderLimit{
			SatelliteId:        satelliteIdentity.ID,
			UplinkPublicKey:    piecePublicKey,
			StorageNodeId:      testrand.NodeID(),
			PieceId:            testrand.PieceID(),
			Action:             pb.PieceAction_PUT,
			SerialNumber:       testrand.SerialNumber(),
			OrderCreation:      time.Now(),
			OrderExpiration:    time.Now().Add(time.Hour),
			PieceExpiration:    time.Now().Add(24 * time.Hour),
			Limit:              1000,
			SatelliteSignature: []byte("invalid signature"),
		}

		signature, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satelliteIdentity), orderLimit)
		require.NoError(t, err)
		orderLimit.SatelliteSignature = signature.SatelliteSignature

		// Should pass with valid signature
		err = trusted.VerifyOrderLimitSignature(untrustedCtx, signing.SigneeFromPeerIdentity(satelliteIdentity.PeerIdentity()), invalidOrderLimit)
		assert.NoError(t, err)
	})

}

func TestAcceptAll_VerifyUplinkOrderSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	none := signaturecheck.AcceptAll{}

	// Create test data (doesn't matter what it is)
	piecePublicKey, _, err := storj.NewPieceKey()
	require.NoError(t, err)

	order := &pb.Order{
		SerialNumber:    testrand.SerialNumber(),
		Amount:          1000,
		UplinkSignature: []byte("invalid signature"),
	}

	// Should always pass
	err = none.VerifyUplinkOrderSignature(ctx, piecePublicKey, order)
	assert.NoError(t, err)
}

func TestAcceptAll_VerifyOrderLimitSignature(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	none := signaturecheck.AcceptAll{}

	// Create test data (doesn't matter what it is)
	piecePublicKey, _, err := storj.NewPieceKey()
	require.NoError(t, err)

	// Create satellite identity
	satelliteIdentity := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())

	require.NoError(t, err)

	orderLimit := &pb.OrderLimit{
		SatelliteId:        satelliteIdentity.ID,
		UplinkPublicKey:    piecePublicKey,
		StorageNodeId:      testrand.NodeID(),
		PieceId:            testrand.PieceID(),
		Action:             pb.PieceAction_PUT,
		SerialNumber:       testrand.SerialNumber(),
		OrderCreation:      time.Now(),
		OrderExpiration:    time.Now().Add(time.Hour),
		PieceExpiration:    time.Now().Add(24 * time.Hour),
		Limit:              1000,
		SatelliteSignature: []byte("invalid signature"),
	}

	// Should always pass
	err = none.VerifyOrderLimitSignature(ctx, signing.SignerFromFullIdentity(satelliteIdentity), orderLimit)
	assert.NoError(t, err)
}
