// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"crypto/tls"
	"io/ioutil"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
)

var (
	mon = monkit.Package()
	// Error is error for tlsopts
	Error = errs.Class("tlsopts error")
)

// Options holds config, identity, and peer verification function data for use with tls.
type Options struct {
	Config   Config
	Ident    *identity.FullIdentity
	RevDB    *identity.RevocationDB
	PCVFuncs []peertls.PeerCertVerificationFunc
	Cert     *tls.Certificate
}

// NewOptions is a constructor for `tls options` given an identity and config
func NewOptions(i *identity.FullIdentity, c Config) (*Options, error) {
	opts := &Options{
		Config: c,
		Ident:  i,
	}

	err := opts.configure(c)
	if err != nil {
		return nil, err
	}

	return opts, nil
}

// configure adds peer certificate verification functions and revocation
// database to the config.
func (opts *Options) configure(c Config) (err error) {
	var pcvs []peertls.PeerCertVerificationFunc
	parseOpts := peertls.ParseExtOptions{}

	if c.UsePeerCAWhitelist {
		whitelist := []byte(DefaultPeerCAWhitelist)
		if c.PeerCAWhitelistPath != "" {
			whitelist, err = ioutil.ReadFile(c.PeerCAWhitelistPath)
			if err != nil {
				return Error.New("unable to find whitelist file %v: %v", c.PeerCAWhitelistPath, err)
			}
		}
		parsed, err := pkcrypto.CertsFromPEM(whitelist)
		if err != nil {
			return Error.Wrap(err)
		}
		parseOpts.CAWhitelist = parsed
		pcvs = append(pcvs, peertls.VerifyCAWhitelist(parsed))
	}

	if c.Extensions.Revocation {
		opts.RevDB, err = identity.NewRevDB(c.RevocationDBURL)
		if err != nil {
			return err
		}
		pcvs = append(pcvs, identity.VerifyUnrevokedChainFunc(opts.RevDB))
	}

	exts := peertls.ParseExtensions(c.Extensions, parseOpts)
	pcvs = append(pcvs, exts.VerifyFunc())

	// NB: remove nil elements
	for i, f := range pcvs {
		if f == nil {
			copy(pcvs[i:], pcvs[i+1:])
			pcvs = pcvs[:len(pcvs)-1]
		}
	}

	opts.PCVFuncs = pcvs

	opts.Cert, err = peertls.TLSCert(opts.Ident.ChainRaw(), opts.Ident.Leaf, opts.Ident.Key)
	return err
}
