// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

// TLSExtConfig is used to bind cli flags for determining which extensions will
// be used by the server
type TLSExtConfig struct {
	Revocation          bool `help:"if true, client leafs may contain the most recent certificate revocation for the current certificate" default:"true"`
	WhitelistSignedLeaf bool `help:"if true, client leafs must contain a valid \"signed certificate extension\" (NB: verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)" default:"false"`
}

// Extensions is a collection of `extension`s for convenience (see `VerifyFunc`)
type Extensions []extension

type extension struct {
	id  asn1.ObjectIdentifier
	f   func(pkix.Extension, [][]*x509.Certificate) (bool, error)
	err error
}

// ParseExtensions an extension config into a slice of extensions with their
// respective ids (`asn1.ObjectIdentifier`) and a function (`f`) which can be
// used in the context of peer certificate verification.
func ParseExtensions(c TLSExtConfig, caWhitelist []*x509.Certificate) (exts Extensions) {
	if c.WhitelistSignedLeaf {
		exts = append(exts, extension{
			id: ExtensionIDs[SignedCertExtID],
			f: func(certExt pkix.Extension, chains [][]*x509.Certificate) (bool, error) {
				if caWhitelist == nil {
					return false, errs.New("whitelist required for leaf whitelist signature verification")
				}
				leaf := chains[0][0]
				for _, ca := range caWhitelist {
					err := VerifySignature(certExt.Value, leaf.RawTBSCertificate, ca.PublicKey)
					if err == nil {
						return true, nil
					}
				}
				return false, nil
			},
			err: ErrVerifyCAWhitelist.New("leaf whitelist signature extension verification error"),
		})
	}

	return exts
}

// VerifyFunc returns a peer certificate verification function which iterates
// over all the leaf cert's extensions and receiver extensions and calls
// `extension#f` when it finds a match by id (`asn1.ObjectIdentifier`)
func (e Extensions) VerifyFunc() PeerCertVerificationFunc {
	return func(_ [][]byte, parsedChains [][]*x509.Certificate) error {
		for _, ext := range parsedChains[0][0].Extensions {
			for _, v := range e {
				if v.id.Equal(ext.Id) {
					ok, err := v.f(ext, parsedChains)
					if err != nil {
						return ErrExtension.Wrap(utils.CombineErrors(v.err, err))
					} else if !ok {
						return v.err
					}
				}
			}
		}
		return nil
	}
}
