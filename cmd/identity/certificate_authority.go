// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/peertls"

	"storj.io/storj/pkg/cfgstruct"
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
	revokeCACmd = &cobra.Command{
		Use:   "revoke",
		Short: "Revoke the identity's leaf certificate (creates backup)",
		RunE:  cmdRevokeCA,
	}

	newCACfg struct {
		CA provider.CASetupConfig
	}

	getIDCfg struct {
		CA provider.PeerCAConfig
	}

	revokeCACfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentityConfig
		// TODO: add "broadcast" option to send revocation to network nodes
	}
)

func init() {
	rootCmd.AddCommand(caCmd)
	caCmd.AddCommand(newCACmd)
	cfgstruct.Bind(newCACmd.Flags(), &newCACfg, cfgstruct.ConfDir(defaultConfDir))
	caCmd.AddCommand(getIDCmd)
	cfgstruct.Bind(getIDCmd.Flags(), &getIDCfg, cfgstruct.ConfDir(defaultConfDir))
	idCmd.AddCommand(revokeCACmd)
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
	extRegex, err := regexp.Compile(filepath.Ext(revokeCACfg.CA.CertPath) + "$")
	backupCfg.CertPath = extRegex.ReplaceAllString(
		revokeCACfg.Identity.CertPath,
		strconv.Itoa(int(time.Now().Unix()))+".bak$1",
	)
	if err := backupCfg.Save(ca); err != nil {
		return err
	}

	if err := peertls.AddRevocationExt(ca.Key, ca.Cert, ca.Cert); err != nil {
		return err
	}

	updateCfg := provider.FullCAConfig{
		CertPath: revokeCACfg.Identity.CertPath,
	}
	if err := updateCfg.Save(ca); err != nil {
		return err
	}
	return nil
}
