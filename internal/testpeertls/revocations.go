// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
)

// RevokeLeaf revokes the leaf certificate in the passed chain and replaces it
// with a "revoking" certificate, which contains a revocation extension recording
// this action.
func RevokeLeaf(keys []crypto.PrivateKey, chain []*x509.Certificate) ([]*x509.Certificate, pkix.Extension, error) {
	if len(chain) < 2 {
		return nil, pkix.Extension{}, errs.New("revoking leaf implies a CA exists; chain too short")
	}
	ca := &identity.FullCertificateAuthority{
		Key:       keys[0],
		Cert:      chain[peertls.CAIndex],
		RestChain: chain[:peertls.CAIndex+1],
	}

	var err error
	ca.ID, err = identity.NodeIDFromKey(ca.Cert.PublicKey)
	if err != nil {
		return nil, pkix.Extension{}, err
	}

	ident := &identity.PeerIdentity{
		Leaf:      chain[peertls.LeafIndex],
		CA:        ca.Cert,
		ID:        ca.ID,
		RestChain: ca.RestChain,
	}

	manageableIdent := identity.NewManageablePeerIdentity(ident, ca)
	if err := manageableIdent.Revoke(); err != nil {
		return nil, pkix.Extension{}, err
	}

	revokingCert := manageableIdent.Leaf
	revocationExt := new(pkix.Extension)
	for _, ext := range revokingCert.Extensions {
		if extensions.RevocationExtID.Equal(ext.Id) {
			*revocationExt = ext
			break
		}
	}
	if revocationExt == nil {
		return nil, pkix.Extension{}, errs.New("no revocation extension found")
	}

	return append([]*x509.Certificate{revokingCert}, chain[peertls.CAIndex:]...), *revocationExt, nil
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
