// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"crypto/x509"
	"io/ioutil"
	"os"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
)

// CertVerificationConfig specifies details around how we verify certificates
type CertVerificationConfig struct {
	RevocationDBURL     string `help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)" default:"bolt://$CONFDIR/revocations.db"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified)"`
	Extensions          peertls.TLSExtConfig
}

// PCVs will return a slice of appropriate Peer Cert Verification functions,
// given a CA whitelist, revocation db, and extension configuration.
func PCVs(caWhitelist []*x509.Certificate, revdb *peertls.RevocationDB,
	exts peertls.TLSExtConfig) (pcvs []peertls.PeerCertVerificationFunc) {
	if len(caWhitelist) > 0 {
		if f := peertls.VerifyCAWhitelist(caWhitelist); f != nil {
			pcvs = append(pcvs, f)
		}
	}
	if revdb != nil {
		if f := peertls.VerifyUnrevokedChainFunc(revdb); f != nil {
			pcvs = append(pcvs, f)
		}
	} else {
		exts.Revocation = false
	}
	if f := peertls.ParseExtensions(exts, peertls.ParseExtOptions{
		CAWhitelist: caWhitelist,
		RevDB:       revdb,
	}).VerifyFunc(); f != nil {
		pcvs = append(pcvs, f)
	}
	return pcvs
}

// Load will return a slice of Peer Cert Verification functions and an opened
// revocation db.
func (cvc CertVerificationConfig) Load() (
	[]peertls.PeerCertVerificationFunc, *peertls.RevocationDB, error) {

	caWhitelist, err := loadWhitelist(cvc.PeerCAWhitelistPath)
	if err != nil {
		return nil, nil, err
	}

	var revdb *peertls.RevocationDB
	if cvc.Extensions.Revocation {
		var err error
		revdb, err = peertls.NewRevDB(cvc.RevocationDBURL)
		if err != nil {
			return nil, nil, err
		}
	}

	return PCVs(caWhitelist, revdb, cvc.Extensions), revdb, nil
}

func loadWhitelist(path string) ([]*x509.Certificate, error) {
	w, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var whitelist []*x509.Certificate
	if w != nil {
		whitelist, err = identity.DecodeAndParseChainPEM(w)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	return whitelist, nil
}
