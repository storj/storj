// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"crypto"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

// PrivateKey implements a signer and signee using a crypto.PrivateKey.
type PrivateKey struct {
	Self storj.NodeID
	Key  crypto.PrivateKey
}

// SignerFromFullIdentity returns signer based on full identity.
func SignerFromFullIdentity(identity *identity.FullIdentity) Signer {
	return &PrivateKey{
		Self: identity.ID,
		Key:  identity.Key,
	}
}

// ID returns node id associated with PrivateKey.
func (private *PrivateKey) ID() storj.NodeID { return private.Self }

// HashAndSign hashes the data and signs with the used key.
func (private *PrivateKey) HashAndSign(data []byte) ([]byte, error) {
	return pkcrypto.HashAndSign(private.Key, data)
}

// HashAndVerifySignature hashes the data and verifies that the signature belongs to the PrivateKey.
func (private *PrivateKey) HashAndVerifySignature(data, signature []byte) error {
	pub := pkcrypto.PublicKeyFromPrivate(private.Key)
	return pkcrypto.HashAndVerifySignature(pub, data, signature)
}

// PublicKey implements a signee using crypto.PublicKey.
type PublicKey struct {
	Self storj.NodeID
	Key  crypto.PublicKey
}

// SigneeFromPeerIdentity returns signee based on peer identity.
func SigneeFromPeerIdentity(identity *identity.PeerIdentity) Signee {
	return &PublicKey{
		Self: identity.ID,
		Key:  identity.Leaf.PublicKey,
	}
}

// ID returns node id associated with this PublicKey.
func (public *PublicKey) ID() storj.NodeID { return public.Self }

// HashAndVerifySignature hashes the data and verifies that the signature belongs to the PublicKey.
func (public *PublicKey) HashAndVerifySignature(data, signature []byte) error {
	return pkcrypto.HashAndVerifySignature(public.Key, data, signature)
}
