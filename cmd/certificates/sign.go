// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
)

var (
	signCmd = &cobra.Command{
		Use:   "sign",
		Short: "Sign a CA and update corresponding CA and identity certificate chains",
		RunE:  cmdSign,
	}

	signCfg struct {
		CA       identity.FullCAConfig
		Identity identity.Config
		// TODO: this default doesn't really make sense
		Signer identity.FullCAConfig
	}
)

func init() {
	rootCmd.AddCommand(signCmd)
	cfgstruct.Bind(signCmd.Flags(), &signCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSign(cmd *cobra.Command, args []string) error {
	ca, err := signCfg.CA.Load()
	if err != nil {
		return err
	}

	ident, err := signCfg.Identity.Load()
	if err != nil {
		return err
	}

	signer, err := signCfg.Signer.Load()
	if err != nil {
		return err
	}
	restChain := []*x509.Certificate{signer.Cert}

	// NB: backup ca and identity
	err = signCfg.CA.SaveBackup(ca)
	if err != nil {
		return err
	}
	err = signCfg.Identity.SaveBackup(ident)
	if err != nil {
		return err
	}

	ca.Cert, err = signer.Sign(ca.Cert)
	if err != nil {
		return err
	}
	ca.RestChain = restChain

	writeErrs := new(errs.Group)
	err = identity.FullCAConfig{
		CertPath: signCfg.CA.CertPath,
	}.Save(ca)
	if err != nil {
		writeErrs.Add(err)
	}

	ident.CA = ca.Cert
	ident.RestChain = restChain

	err = identity.Config{
		CertPath: signCfg.Identity.CertPath,
	}.Save(ident)
	if err != nil {
		writeErrs.Add(err)
	}

	return writeErrs.Err()
}
