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
	Config            Config
	Ident             *identity.FullIdentity
	RevDB             *identity.RevocationDB
	VerificationFuncs *VerificationFuncs
	Cert              *tls.Certificate
}

// VerificationFuncs keeps track of of client and server peer certificate verification
// functions for use in tls handshakes.
type VerificationFuncs struct {
	client []peertls.PeerCertVerificationFunc
	server []peertls.PeerCertVerificationFunc
}

// NewOptions is a constructor for `tls options` given an identity and config
func NewOptions(i *identity.FullIdentity, c Config) (*Options, error) {
	opts := &Options{
		Config: c,
		Ident:  i,
		VerificationFuncs: new(VerificationFuncs),
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
		opts.VerificationFuncs.ClientAdd(peertls.VerifyCAWhitelist(parsed))
	}

	if c.Extensions.Revocation {
		opts.RevDB, err = identity.NewRevDB(c.RevocationDBURL)
		if err != nil {
			return err
		}
		opts.VerificationFuncs.Add(identity.VerifyUnrevokedChainFunc(opts.RevDB))
	}

	exts := peertls.ParseExtensions(c.Extensions, parseOpts)
	opts.VerificationFuncs.Add(exts.VerifyFunc())

	opts.Cert, err = peertls.TLSCert(opts.Ident.RawChain(), opts.Ident.Leaf, opts.Ident.Key)
	return err
}

// Client returns the client verification functions.
func (vf *VerificationFuncs) Client() []peertls.PeerCertVerificationFunc {
	return vf.client
}

// Server returns the server verification functions.
func (vf *VerificationFuncs) Server() []peertls.PeerCertVerificationFunc {
	return vf.server
}

// Add adds verification functions so the client and server lists.
func (vf *VerificationFuncs) Add(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	vf.ClientAdd(verificationFuncs...)
	vf.ServerAdd(verificationFuncs...)
}

// ClientAdd adds verification functions so the client list.
func (vf *VerificationFuncs) ClientAdd(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	verificationFuncs = removeNils(verificationFuncs)
	vf.client = append(vf.client, verificationFuncs...)
}

// ServerAdd adds verification functions so the server list.
func (vf *VerificationFuncs) ServerAdd(verificationFuncs ...peertls.PeerCertVerificationFunc) {
	verificationFuncs = removeNils(verificationFuncs)
	vf.server = append(vf.server, verificationFuncs...)
}

func removeNils(verificationFuncs []peertls.PeerCertVerificationFunc) []peertls.PeerCertVerificationFunc {
	for i, f := range verificationFuncs {
		if f == nil {
			copy(verificationFuncs[i:], verificationFuncs[i+1:])
			verificationFuncs = verificationFuncs[:len(verificationFuncs)-1]
		}
	}
	return verificationFuncs
}
