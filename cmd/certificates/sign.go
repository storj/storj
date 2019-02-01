// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
)

var (
	signCmd = &cobra.Command{
		Use:   "sign [signee identity-dir]",
		Short: "Sign a CA and update corresponding CA and identity certificate chains",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdSign,
	}

	signCfg struct {
		signeeCACfg    identity.PeerCAConfig
		signeeIdentCfg identity.PeerConfig
		// NB: defaults to same as CA
		Signer identity.FullCAConfig
	}
)

func init() {
	rootCmd.AddCommand(signCmd)
	cfgstruct.Bind(signCmd.Flags(), &signCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdSign(cmd *cobra.Command, args []string) error {
	ca, err := signCfg.signeeCACfg.Load()
	if err != nil {
		return err
	}

	ident, err := signCfg.signeeIdentCfg.Load()
	if err != nil {
		log.Printf("unable to load identity: %s\n", err.Error())
	}

	signer, err := signCfg.Signer.Load()
	if err != nil {
		fmt.Println("two")
		return err
	}
	restChain := []*x509.Certificate{signer.Cert}

	// NB: backup ca and identity
	err = signCfg.signeeCACfg.SaveBackup(ca)
	if err != nil {
		return err
	}
	err = signCfg.signeeIdentCfg.SaveBackup(ident)
	if err != nil {
		log.Printf("unable to save backup of identity: %s\n", err.Error())
	}

	ca.Cert, err = signer.Sign(ca.Cert)
	if err != nil {
		return err
	}
	ca.RestChain = restChain

	writeErrs := new(errs.Group)
	err = signCfg.signeeCACfg.Save(ca)
	if err != nil {
		writeErrs.Add(err)
	}

	ident.CA = ca.Cert
	ident.RestChain = restChain

	err = signCfg.signeeIdentCfg.Save(ident)
	if err != nil {
		log.Printf("unable to save identity: %s\n", err.Error())
	}

	return writeErrs.Err()
}
