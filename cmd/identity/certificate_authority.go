// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	caCmd = &cobra.Command{
		Use:   "ca",
		Short: "Manage certificate authorities",
	}

	newCACmd = &cobra.Command{
		Use:   "new",
		Short: "Create a new certificate authority",
		RunE:  cmdNewCA,
	}

	getIDCmd = &cobra.Command{
		Use:   "id",
		Short: "Get the id of a CA",
		RunE:  cmdGetID,
	}

	caExtCmd = &cobra.Command{
		Use:   "extensions",
		Short: "Prints the extensions attached to the identity CA certificate",
		RunE:  cmdCAExtensions,
	}
	revokeCACmd = &cobra.Command{
		Use:   "revoke",
		Short: "Revoke the identity's CA certificate (creates backup)",
		RunE:  cmdRevokeCA,
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
	cfgstruct.Bind(newCACmd.Flags(), &newCACfg, cfgstruct.ConfDir(defaultConfDir))
	caCmd.AddCommand(getIDCmd)
	cfgstruct.Bind(getIDCmd.Flags(), &getIDCfg, cfgstruct.ConfDir(defaultConfDir))
	caCmd.AddCommand(caExtCmd)
	cfgstruct.Bind(caExtCmd.Flags(), &caExtCfg, cfgstruct.ConfDir(defaultConfDir))
	caCmd.AddCommand(revokeCACmd)
	cfgstruct.Bind(revokeCACmd.Flags(), &revokeCACfg, cfgstruct.ConfDir(defaultConfDir))
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
	var backupCfg provider.FullCAConfig
	certPathExt := filepath.Ext(revokeCACfg.CA.CertPath)
	certPath := revokeCACfg.CA.CertPath
	certBase := certPath[:len(certPath)-len(certPathExt)]
	if err != nil {
		return err
	}
	backupCfg.CertPath = fmt.Sprintf(
		"%s.%s%s",
		certBase,
		strconv.Itoa(int(time.Now().Unix())),
		certPathExt,
	)
	if err := backupCfg.Save(ca); err != nil {
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
