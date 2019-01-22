// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
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

	cfgstruct.Bind(newCACmd.Flags(), &newCACfg, cfgstruct.CredsDir(defaultIdentityDir))
	cfgstruct.Bind(getIDCmd.Flags(), &getIDCfg, cfgstruct.CredsDir(defaultIdentityDir))
	cfgstruct.Bind(caExtCmd.Flags(), &caExtCfg, cfgstruct.CredsDir(defaultIdentityDir))
	cfgstruct.Bind(revokeCACmd.Flags(), &revokeCACfg, cfgstruct.CredsDir(defaultIdentityDir))
}

func cmdNewCA(cmd *cobra.Command, args []string) error {
	_, err := newCACfg.CA.Create(process.Ctx(cmd))
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
