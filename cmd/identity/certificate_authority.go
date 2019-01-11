// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	caCmd = &cobra.Command{
		Use:         "ca",
		Short:       "Manage certificate authorities",
		Annotations: map[string]string{"type": "setup"},
	}

	newCACmd = &cobra.Command{
		Use:         "new",
		Short:       "Create a new certificate authority",
		RunE:        cmdNewCA,
		Annotations: map[string]string{"type": "setup"},
	}

	getIDCmd = &cobra.Command{
		Use:         "id",
		Short:       "Get the id of a CA",
		RunE:        cmdGetID,
		Annotations: map[string]string{"type": "setup"},
	}

	caExtCmd = &cobra.Command{
		Use:         "extensions",
		Short:       "Prints the extensions attached to the identity CA certificate",
		RunE:        cmdCAExtensions,
		Annotations: map[string]string{"type": "setup"},
	}
	revokeCACmd = &cobra.Command{
		Use:         "revoke",
		Short:       "Revoke the identity's CA certificate (creates backup)",
		RunE:        cmdRevokeCA,
		Annotations: map[string]string{"type": "setup"},
	}

	newCACfg struct {
		CA provider.CASetupConfig
	}

	getIDCfg struct {
		CA provider.PeerCAConfig
	}

	caExtCfg struct {
		CA provider.FullCAConfig
	}

	revokeCACfg struct {
		CA provider.FullCAConfig
		// TODO: add "broadcast" option to send revocation to network nodes
	}
)

func init() {
	rootCmd.AddCommand(caCmd)

	caCmd.AddCommand(newCACmd)
	caCmd.AddCommand(getIDCmd)
	caCmd.AddCommand(caExtCmd)
	caCmd.AddCommand(revokeCACmd)

	cfgstruct.Bind(newCACmd.Flags(), &newCACfg, cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.Bind(getIDCmd.Flags(), &getIDCfg, cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.Bind(caExtCmd.Flags(), &caExtCfg, cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.Bind(revokeCACmd.Flags(), &revokeCACfg, cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdNewCA(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	if _, err = os.Stat(newCACfg.CA.KeyPath); !os.IsNotExist(err) {
		var (
			parent *identity.FullCertificateAuthority
			opts   identity.NewCAOptions
		)
		if newCACfg.CA.ParentCertPath != "" && newCACfg.CA.ParentKeyPath != "" {
			parent, err = identity.FullCAConfig{
				CertPath: newCACfg.CA.ParentCertPath,
				KeyPath:  newCACfg.CA.ParentKeyPath,
			}.Load()
		}
		if parent != nil {
			opts = identity.NewCAOptions{
				ParentCert: parent.Cert,
				ParentKey:  parent.Key,
			}
		}

		kb, err := ioutil.ReadFile(newCACfg.CA.KeyPath)
		if err != nil {
			return peertls.ErrNotExist.Wrap(err)
		}
		kp, _ := pem.Decode(kb)
		key, err := x509.ParseECPrivateKey(kp.Bytes)
		if err != nil {
			return errs.New("unable to parse EC private key: %v", err)
		}

		ca, err := identity.NewCAFromKey(key, opts)
		if err != nil {
			return err
		}
		err = newCACfg.CA.FullConfig().Save(ca)
	} else {
		_, err = newCACfg.CA.Create(ctx)
	}
	return err
}

func cmdGetID(cmd *cobra.Command, args []string) (err error) {
	p, err := getIDCfg.CA.Load()
	if err != nil {
		return err
	}

	fmt.Println(p.ID.String())
	return nil
}

func cmdRevokeCA(cmd *cobra.Command, args []string) (err error) {
	ca, err := revokeCACfg.CA.Load()
	if err != nil {
		return err
	}

	// NB: backup original cert
	if err := revokeCACfg.CA.SaveBackup(ca); err != nil {
		return err
	}

	if err := peertls.AddRevocationExt(ca.Key, ca.Cert, ca.Cert); err != nil {
		return err
	}

	updateCfg := provider.FullCAConfig{
		CertPath: revokeCACfg.CA.CertPath,
	}
	if err := updateCfg.Save(ca); err != nil {
		return err
	}
	return nil
}

func cmdCAExtensions(cmd *cobra.Command, args []string) (err error) {
	ca, err := caExtCfg.CA.Load()
	if err != nil {
		return err
	}

	return printExtensions(ca.Cert.Raw, ca.Cert.ExtraExtensions)
}
