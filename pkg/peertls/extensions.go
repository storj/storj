package peertls

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

type TLSExtConfig struct {
	Revocation          bool `help:"if true, client leafs may contain the most recent certificate revocation for the current certificate" default:"true"`
	WhitelistSignedLeaf bool `help:"if true, client leafs must contain a valid \"authority signature extension\" (NB: authority signature extensions are verified against certs in the peer ca whitelist; i.e. if true, a whitelist must be provided)" default:"false"`
}

type Extensions []extension

type extension struct {
	id  asn1.ObjectIdentifier
	f   func(pkix.Extension, [][]*x509.Certificate) (bool, error)
	err error
}

func ParseExtensions(c TLSExtConfig, caWhitelist []*x509.Certificate) (exts Extensions) {
	if c.WhitelistSignedLeaf {
		exts = append(exts, extension{
			id: ExtensionIDs[WhitelistSignedLeafExtID],
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

	// if c.Revocation {
	// 	exts = append(exts, extension{
	// 		id: ExtensionIDs[RevocationExtID],
	// 		f: func(certExt pkix.Extension, chains [][]*x509.Certificate) (bool, error) {
	// 			ca := chains[0][1]
	// 			// verify timestamp is >
	// 			// verify revocation signed by ca
	// 		},
	// 		// err: ErrRevocation.New("")
	// 	})
	// }

	return exts
}

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

