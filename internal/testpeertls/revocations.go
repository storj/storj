// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
)

// RevokeLeaf revokes the leaf certificate in the passed chain and replaces it
// with a "revoking" certificate, which contains a revocation extension recording
// this action.
func RevokeLeaf(keys []crypto.PrivateKey, chain []*x509.Certificate) ([]*x509.Certificate, pkix.Extension, error) {
	var revocation pkix.Extension
	revokingKey, err := pkcrypto.GeneratePrivateKey()
	if err != nil {
		return nil, revocation, err
	}

	revokingTemplate, err := peertls.LeafTemplate()
	if err != nil {
		return nil, revocation, err
	}

	revokingPubKey := pkcrypto.PublicKeyFromPrivate(revokingKey)
	revokingCert, err := peertls.CreateCertificate(revokingPubKey, keys[0], revokingTemplate, chain[peertls.CAIndex])
	if err != nil {
		return nil, revocation, err
	}

	err = extensions.AddRevocationExt(keys[0], chain[peertls.LeafIndex], revokingCert)
	if err != nil {
		return nil, revocation, err
	}

	revocation = revokingCert.ExtraExtensions[0]
	return append([]*x509.Certificate{revokingCert}, chain[peertls.CAIndex:]...), revocation, nil
}

// RevokeCA revokes the CA certificate in the passed chain and adds a revocation
// extension to that certificate, recording this action.
func RevokeCA(keys []crypto.PrivateKey, chain []*x509.Certificate) ([]*x509.Certificate, pkix.Extension, error) {
	caCert := chain[peertls.CAIndex]
	err := extensions.AddRevocationExt(keys[0], caCert, caCert)
	if err != nil {
		return nil, pkix.Extension{}, err
	}

	return append([]*x509.Certificate{caCert}, chain[peertls.CAIndex:]...), caCert.ExtraExtensions[0], nil
}

// NewRevokedLeafChain creates a certificate chain (of length 2) with a leaf
// that contains a valid revocation extension.
func NewRevokedLeafChain() ([]crypto.PrivateKey, []*x509.Certificate, pkix.Extension, error) {
	keys, certs, err := NewCertChain(2)
	if err != nil {
		return nil, nil, pkix.Extension{}, err
	}

	newChain, revocation, err := RevokeLeaf(keys, certs)
	return keys, newChain, revocation, err
}
