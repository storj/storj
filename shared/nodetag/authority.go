// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetag

import (
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
)

var (
	// UnknownSignee is returned when the public key is not available for NodeID to check the signature.
	UnknownSignee = errs.Class("node tag signee is unknown")
)

// Authority contains all possible signee.
type Authority []signing.Signee

// Verify checks if any of the storage signee can validate the signature.
func (a Authority) Verify(ctx context.Context, tags *pb.SignedNodeTagSet) (*pb.NodeTagSet, error) {
	for _, signee := range a {
		if bytes.Equal(signee.ID().Bytes(), tags.SignerNodeId) {
			return Verify(ctx, tags, signee)
		}
	}
	return nil, UnknownSignee.New("no certificate for signer nodeID: %x", tags.SignerNodeId)
}

// Config is a config for self-signed nodetags.
type Config struct {
	TagAuthorities string `help:"comma-separated paths of additional cert files, used to validate signed node tags"`
}

// LoadAuthorities is loading node authorities.
func LoadAuthorities(peerIdentity *identity.PeerIdentity, authorityLocations string) (Authority, error) {
	var authority Authority
	authority = append(authority, signing.SigneeFromPeerIdentity(peerIdentity))
	for _, cert := range strings.Split(authorityLocations, ",") {
		cert = strings.TrimSpace(cert)
		if cert == "" {
			continue
		}
		cert = strings.TrimSpace(cert)
		raw, err := os.ReadFile(cert)
		if err != nil {
			return nil, errs.New("Couldn't load identity for node tag authority from %s: %v", cert, err)
		}
		pi, err := identity.PeerIdentityFromPEM(raw)
		if err != nil {
			return nil, errs.New("Node tag authority file  %s couldn't be loaded as peer identity: %v", cert, err)
		}
		authority = append(authority, signing.SigneeFromPeerIdentity(pi))
	}
	return authority, nil
}
