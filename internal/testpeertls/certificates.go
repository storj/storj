// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"crypto"
	"crypto/x509"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

// NewCertChain creates a valid peertls certificate chain (and respective keys) of the desired length.
func NewCertChain(length int, versionNumber storj.IDVersionNumber) (keys []crypto.PrivateKey, certs []*x509.Certificate, _ error) {
	version, err := storj.GetIDVersion(versionNumber)
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < length; i++ {
		key, err := pkcrypto.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}
		keys = append([]crypto.PrivateKey{key}, keys...)

		var template *x509.Certificate
		if i == 0 {
			template, err = peertls.CATemplate()
			if err = extensions.AddExtraExtension(template, storj.NewVersionExt(version)); err != nil {
				return nil, nil, err
			}
		} else {
			template, err = peertls.LeafTemplate()
		}
		if err != nil {
			return nil, nil, err
		}

		var cert *x509.Certificate
		if i == 0 {
			cert, err = peertls.NewSelfSignedCert(key, template)
		} else {
			// NB: 	`keys[1]`: key has already been prepended; parent key is at first index
			// 		`certs[0]`: cert hasn't been prepended yet; parent cert is at zeroth index
			cert, err = peertls.NewCert(pkcrypto.PublicKeyFromPrivate(key), keys[1], template, certs[0])
		}
		if err != nil {
			return nil, nil, err
		}

		certs = append([]*x509.Certificate{cert}, certs...)
	}
	return keys, certs, nil
}
