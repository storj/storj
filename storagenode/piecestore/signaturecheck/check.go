// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package signaturecheck

import (
	"context"

	"github.com/zeebo/errs/v2"
	"golang.org/x/exp/slices"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
)

// Check defines the interface for verifying order and order limit signatures.
type Check interface {
	VerifyUplinkOrderSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.Order) error
	VerifyOrderLimitSignature(ctx context.Context, satellite signing.Signee, signed *pb.OrderLimit) error
}

// Full implements the Check interface and performs full signature verification.
type Full struct {
}

// VerifyUplinkOrderSignature verifies the signature of an order from an uplink.
func (f *Full) VerifyUplinkOrderSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.Order) error {
	return signing.VerifyUplinkOrderSignature(ctx, publicKey, signed)
}

// VerifyOrderLimitSignature verifies the signature of an order limit from a satellite.
func (f *Full) VerifyOrderLimitSignature(ctx context.Context, satellite signing.Signee, signed *pb.OrderLimit) error {
	return signing.VerifyOrderLimitSignature(ctx, satellite, signed)
}

var _ Check = (*Full)(nil)

// Config holds the configuration for the Trusted signature checker.
type Config struct {
	TrustedUplinks []string `usage:"List of trusted node IDs for signature verification. These nodes will bypass signature checks."`
}

// Trusted implements the Check interface and bypasses signature verification
// for uplinks and satellites whose node IDs are in the trusted list.
type Trusted struct {
	trusted []storj.NodeID
}

// NewTrusted creates a new Trusted signature checker.
func NewTrusted(config Config) (*Trusted, error) {
	trusted := make([]storj.NodeID, 0, len(config.TrustedUplinks))
	for _, nodeID := range config.TrustedUplinks {
		if nodeID == "" {
			continue
		}
		id, err := storj.NodeIDFromString(nodeID)
		if err != nil {
			return nil, errs.Errorf("Couldn't parse node ID %q for trusted signature check: %v", nodeID, err)
		}
		trusted = append(trusted, id)
	}
	return &Trusted{
		trusted: trusted,
	}, nil
}

// VerifyUplinkOrderSignature verifies the signature of an order from an uplink.
// If the peer ID from the context is in the trusted list, signature verification is skipped.
func (t *Trusted) VerifyUplinkOrderSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.Order) error {
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(t.trusted, peer.ID) {
		return nil
	}
	return signing.VerifyUplinkOrderSignature(ctx, publicKey, signed)
}

// VerifyOrderLimitSignature verifies the signature of an order limit from a satellite.
// If the peer ID from the context is in the trusted list, signature verification is skipped.
func (t *Trusted) VerifyOrderLimitSignature(ctx context.Context, satellite signing.Signee, signed *pb.OrderLimit) error {
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(t.trusted, peer.ID) {
		return nil
	}
	return signing.VerifyOrderLimitSignature(ctx, satellite, signed)
}

var _ Check = (*Trusted)(nil)

// AcceptAll implements the Check interface and performs no signature verification.
// All signature checks will pass.
type AcceptAll struct {
}

// VerifyUplinkOrderSignature always returns nil, effectively skipping signature verification.
func (n AcceptAll) VerifyUplinkOrderSignature(ctx context.Context, publicKey storj.PiecePublicKey, signed *pb.Order) error {
	return nil
}

// VerifyOrderLimitSignature always returns nil, effectively skipping signature verification.
func (n AcceptAll) VerifyOrderLimitSignature(ctx context.Context, satellite signing.Signee, signed *pb.OrderLimit) error {
	return nil
}

var _ Check = (*AcceptAll)(nil)
