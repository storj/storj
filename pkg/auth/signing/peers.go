// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"crypto"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

type PrivateKey struct {
	Self storj.NodeID
	Key  crypto.PrivateKey
}

func SignerFromFullIdentity(identity *identity.FullIdentity) Signer {
	return &PrivateKey{
		Self: identity.ID,
		Key:  identity.Key,
	}
}

func (private *PrivateKey) ID() storj.NodeID { return private.Self }

func (private *PrivateKey) HashAndSign(data []byte) ([]byte, error) {
	return pkcrypto.HashAndSign(private.Key, data)
}

func (private *PrivateKey) HashAndVerifySignature(data, signature []byte) error {
	pub := pkcrypto.PublicKeyFromPrivate(private.Key)
	return pkcrypto.HashAndVerifySignature(pub, data, signature)
}

type PublicKey struct {
	Self storj.NodeID
	Key  crypto.PublicKey
}

func SigneeFromPeerIdentity(identity *identity.PeerIdentity) Signee {
	return &PublicKey{
		Self: identity.ID,
		Key:  identity.Leaf.PublicKey,
	}
}
func (public *PublicKey) ID() storj.NodeID { return public.Self }

func (public *PublicKey) HashAndVerifySignature(data, signature []byte) error {
	return pkcrypto.HashAndVerifySignature(public.Key, data, signature)
}
