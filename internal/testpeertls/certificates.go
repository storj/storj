// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testpeertls

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

// NewCertChain creates a valid peertls certificate chain (and respective keys) of the desired length.
// NB: keys are in the reverse order compared to certs (i.e. first key belongs to last cert)!
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
		// TODO: get your head straight with the keys and certs slices
		keys = append(keys, key)

		var template *x509.Certificate
		if i == length-1 {
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
		if i == length-1 {
			cert, err = peertls.NewCert(pkcrypto.PublicKeyFromPrivate(key), keys[i-1], template, certs[i-1:][0])
		} else {
			cert, err = peertls.NewSelfSignedCert(key, template)
		}
		if err != nil {
			return nil, nil, err
		}

		fmt.Printf("i %d extensions: %+v\n", i, cert.Extensions)
		certs = append(certs, cert)
	}
	for i, cert := range certs {
		fmt.Println(i)
		fmt.Println(cert.IsCA)
	}
	return keys, certs, nil
}
