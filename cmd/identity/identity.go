// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	// ErrSetup is used when an error occurs while setting up
	ErrSetup = errs.Class("setup error")

	idCmd = &cobra.Command{
		Use:         "id",
		Short:       "Manage identities",
		Annotations: map[string]string{"type": "setup"},
	}

	newIDCmd = &cobra.Command{
		Use:         "create",
		Short:       "Creates a new identity from an existing certificate authority",
		RunE:        cmdNewID,
		Annotations: map[string]string{"type": "setup"},
	}

	leafExtCmd = &cobra.Command{
		Use:         "extensions",
		Short:       "Prints the extensions attached to the identity leaf certificate",
		Args:        cobra.MaximumNArgs(1),
		RunE:        cmdLeafExtensions,
		Annotations: map[string]string{"type": "setup"},
	}

	revokeLeafCmd = &cobra.Command{
		Use:         "revoke",
		Short:       "Revoke the identity's leaf certificate (creates backup)",
		RunE:        cmdRevokeLeaf,
		Annotations: map[string]string{"type": "setup"},
	}

	newIDCfg struct {
		CA       identity.FullCAConfig
		Identity identity.SetupConfig
	}

	leafExtCfg struct {
		Identity identity.PeerConfig
	}

	revokeLeafCfg struct {
		CA       identity.FullCAConfig
		Identity identity.Config
		// TODO: add "broadcast" option to send revocation to network nodes
	}
)

func init() {
	rootCmd.AddCommand(idCmd)
	idCmd.AddCommand(newIDCmd)
	idCmd.AddCommand(leafExtCmd)
	idCmd.AddCommand(revokeLeafCmd)

	process.Bind(newIDCmd, &newIDCfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(leafExtCmd, &leafExtCfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(revokeLeafCmd, &revokeLeafCfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdNewID(cmd *cobra.Command, args []string) (err error) {
	ca, err := newIDCfg.CA.Load()
	if err != nil {
		return err
	}

	s, err := newIDCfg.Identity.Status()
	if err != nil {
		return err
	}
	if s == identity.NoCertNoKey || newIDCfg.Identity.Overwrite {
		_, err := newIDCfg.Identity.Create(ca)
		return err
	}
	return ErrSetup.New("identity file(s) exist: %s", s)
}

func cmdLeafExtensions(cmd *cobra.Command, args []string) (err error) {
	if len(args) > 0 {
		leafExtCfg.Identity = identity.PeerConfig{
			CertPath: filepath.Join(identityDir, args[0], "identity.cert"),
		}
	}

	ident, err := leafExtCfg.Identity.Load()
	if err != nil {
		return err
	}

	return printExtensions(ident.Leaf.Raw, ident.Leaf.Extensions)
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

	manageableIdent := identity.NewManageableFullIdentity(originalIdent, ca)
	if err := manageableIdent.Revoke(); err != nil {
		return err
	}

	// NB: backup original cert and key.
	if err := revokeLeafCfg.Identity.SaveBackup(originalIdent); err != nil {
		return err
	}

	if err := revokeLeafCfg.Identity.Save(manageableIdent.FullIdentity); err != nil {
		return err
	}
	return nil
}
