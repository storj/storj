// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"crypto/x509"
	"io/ioutil"
	"os"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
)

// Options holds config, identity, and peer verification function data for use with a grpc server.
type Options struct {
	Config   Config
	Ident    *identity.FullIdentity
	RevDB    *peertls.RevocationDB
	PCVFuncs []peertls.PeerCertVerificationFunc
}

// NewOptions is a constructor for `serverOptions` given an identity and config
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

func (opts *Options) grpcOpts() (grpc.ServerOption, error) {
	return opts.Ident.ServerOption(opts.PCVFuncs...)
}

// configure adds peer certificate verification functions and revocation
// database to the config.
func (opts *Options) configure(c Config) (err error) {
	var pcvs []peertls.PeerCertVerificationFunc
	parseOpts := peertls.ParseExtOptions{}

	if c.PeerCAWhitelistPath != "" {
		caWhitelist, err := loadWhitelist(c.PeerCAWhitelistPath)
		if err != nil {
			return err
		}
		parseOpts.CAWhitelist = caWhitelist
		pcvs = append(pcvs, peertls.VerifyCAWhitelist(caWhitelist))
	}

	if c.Extensions.Revocation {
		opts.RevDB, err = peertls.NewRevDB(c.RevocationDBURL)
		if err != nil {
			return err
		}
		pcvs = append(pcvs, peertls.VerifyUnrevokedChainFunc(opts.RevDB))
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
	return nil
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
