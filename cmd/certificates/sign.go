// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
)

var (
	signCmd = &cobra.Command{
		Use:   "sign [signee identity-dir]",
		Short: "Sign a CA and update corresponding CA and identity certificate chains",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdSign,
	}

	signCfg struct {
		SigneeCACfg    identity.PeerCAConfig
		SigneeIdentCfg identity.PeerConfig
		// NB: defaults to same as CA
		Signer identity.FullCAConfig
	}
)

func cmdSign(cmd *cobra.Command, args []string) error {
	ca, err := signCfg.SigneeCACfg.Load()
	if err != nil {
		return err
	}

	var (
		signeeIdentityExists bool
		ident                *identity.PeerIdentity
	)
	_, err = os.Stat(signCfg.SigneeIdentCfg.CertPath)
	if err != nil {
		signeeIdentityExists = !os.IsNotExist(err)
		if signeeIdentityExists {
			return err
		}

		ident, err = signCfg.SigneeIdentCfg.Load()
		if err != nil {
			return err
		}
	}

	signer, err := signCfg.Signer.Load()
	if err != nil {
		return err
	}
	restChain := []*x509.Certificate{signer.Cert}

	// NB: backup ca and identity
	err = signCfg.SigneeCACfg.SaveBackup(ca)
	if err != nil {
		return err
	}

	ca.Cert, err = signer.Sign(ca.Cert)
	if err != nil {
		return err
	}
	ca.RestChain = restChain

	writeErrs := new(errs.Group)
	err = signCfg.SigneeCACfg.Save(ca)
	if err != nil {
		writeErrs.Add(err)
	}

	if signeeIdentityExists {
		err = signCfg.SigneeIdentCfg.SaveBackup(ident)
		if err != nil {
			writeErrs.Add(err)
		}
		ident.CA = ca.Cert
		ident.RestChain = restChain

		err = signCfg.SigneeIdentCfg.Save(ident)
		if err != nil {
			writeErrs.Add(err)
		}
	}

	return writeErrs.Err()
}
