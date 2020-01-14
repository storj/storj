// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"crypto/x509"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
)

type verifyConfig struct {
	CA       identity.FullCAConfig
	Identity identity.Config
	Signer   identity.FullCAConfig
}

var (
	errVerify = errs.Class("Verify error")
	verifyCmd = &cobra.Command{
		Use:   "verify",
		Short: "Verify identity and CA certificate chains are valid",
		Long:  "Verify identity and CA certificate chains are valid.\n\nTo be valid, an identity certificate chain must contain its CA's certificate chain, and both chains must consist of certificates signed by their respective parents, ending in a self-signed root.",
		RunE:  cmdVerify,
	}

	verifyCfg verifyConfig
)

type checkOpts struct {
	verifyConfig
	ca       *identity.FullCertificateAuthority
	identity *identity.FullIdentity
	errGroup *errs.Group
}

func cmdVerify(cmd *cobra.Command, args []string) error {
	ca, err := verifyCfg.CA.Load()
	if err != nil {
		return err
	}

	ident, err := verifyCfg.Identity.Load()
	if err != nil {
		return err
	}

	opts := checkOpts{
		verifyConfig: verifyCfg,
		ca:           ca,
		identity:     ident,
		errGroup:     new(errs.Group),
	}
	checks := []struct {
		errFmt string
		run    func(checkOpts, string)
	}{
		{
			errFmt: "identity chain must contain CA chain: %s",
			run:    checkIdentContainsCA,
		},
		{
			errFmt: "identity chain must be valid: %s",
			run:    checkIdentChain,
		},
		{
			errFmt: "CA chain must be valid: %s",
			run:    checkCAChain,
		},
	}

	for _, check := range checks {
		check.run(opts, check.errFmt)
	}

	return opts.errGroup.Err()
}

func checkIdentChain(opts checkOpts, errFmt string) {
	identChain := append([]*x509.Certificate{
		opts.identity.Leaf,
		opts.identity.CA,
	}, opts.identity.RestChain...)

	verifyChain(identChain, errFmt, opts.errGroup)
}

func checkCAChain(opts checkOpts, errFmt string) {
	caChain := append([]*x509.Certificate{
		opts.ca.Cert,
	}, opts.ca.RestChain...)

	verifyChain(caChain, errFmt, opts.errGroup)
}

func checkIdentContainsCA(opts checkOpts, errFmt string) {
	identChainBytes := opts.identity.RawChain()
	caChainBytes := opts.ca.RawChain()

	for i, caCert := range caChainBytes {
		j := i + 1
		if len(identChainBytes) == j {
			opts.errGroup.Add(errVerify.New(errFmt, "identity chain should be longer than ca chain"))
			break
		}
		if !bytes.Equal(caCert, identChainBytes[j]) {
			opts.errGroup.Add(errVerify.New(errFmt,
				fmt.Sprintf("identity and ca chains don't match at indicies %d and %d, respectively", j, i),
			))
		}
	}
}

func verifyChain(chain []*x509.Certificate, errFormat string, errGroup *errs.Group) {
	for i, cert := range chain {
		if i+1 == len(chain) {
			break
		}
		parent := chain[i+1]

		if err := cert.CheckSignatureFrom(parent); err != nil {
			errGroup.Add(errs.New(errFormat, err))
			break

		}
	}
}
