// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/peertls"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/provider"
)

var (
	idCmd = &cobra.Command{
		Use:   "id",
		Short: "Manage identities",
	}

	newIDCmd = &cobra.Command{
		Use:   "new",
		Short: "Creates a new identity from an existing certificate authority",
		RunE:  cmdNewID,
	}
	revokeLeafCmd = &cobra.Command{
		Use:   "revoke",
		Short: "Revoke the identity's leaf certificate (creates backup)",
		RunE:  cmdRevokeLeaf,
	}

	newIDCfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentitySetupConfig
	}

	revokeLeafCfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentityConfig
		// TODO: add "broadcast" option to send revocation to network nodes
	}
)

func init() {
	rootCmd.AddCommand(idCmd)
	idCmd.AddCommand(newIDCmd)
	cfgstruct.Bind(newIDCmd.Flags(), &newIDCfg, cfgstruct.ConfDir(defaultConfDir))
	idCmd.AddCommand(revokeLeafCmd)
	cfgstruct.Bind(revokeLeafCmd.Flags(), &revokeLeafCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdNewID(cmd *cobra.Command, args []string) (err error) {
	ca, err := newIDCfg.CA.Load()
	if err != nil {
		return err
	}

	s := newIDCfg.Identity.Stat()
	if s == provider.NoCertNoKey || newIDCfg.Identity.Overwrite {
		_, err := newIDCfg.Identity.Create(ca)
		return err
	}
	return provider.ErrSetup.New("identity file(s) exist: %s", s)
}

func cmdRevokeLeaf(cmd *cobra.Command, args []string) (err error) {
	ca, err := revokeLeafCfg.CA.Load()
	if err != nil {
		return err
	}
	originalIdent, err := revokeLeafCfg.Identity.Load()
	if err != nil {
		return err
	}

	updatedIdent, err := ca.NewIdentity()
	if err != nil {
		return err
	}

	if err := peertls.AddRevocationExt(ca.Key, originalIdent.Leaf, updatedIdent.Leaf); err != nil {
		return err
	}

	// NB: backup original cert
	var backupCfg provider.IdentityConfig
	extRegex, err := regexp.Compile(filepath.Ext(revokeLeafCfg.Identity.CertPath) + "$")
	if err != nil {
		return err
	}
	backupCfg.CertPath = extRegex.ReplaceAllString(
		revokeLeafCfg.Identity.CertPath,
		strconv.Itoa(int(time.Now().Unix()))+".bak$1",
	)
	if err := backupCfg.Save(updatedIdent); err != nil {
		return err
	}

	updateCfg := provider.IdentityConfig{
		CertPath: revokeLeafCfg.Identity.CertPath,
	}
	if err := updateCfg.Save(updatedIdent); err != nil {
		return err
	}
	return nil
}
