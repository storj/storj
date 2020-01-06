// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/revocation"
)

var (
	caCmd = &cobra.Command{
		Use:         "certificate-authority",
		Short:       "Manage certificate authorities",
		Annotations: map[string]string{"type": "setup"},
	}

	newCACmd = &cobra.Command{
		Use:         "create",
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
		Use:         "extensions [service]",
		Short:       "Prints the extensions attached to the identity CA certificate",
		RunE:        cmdCAExtensions,
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"type": "setup"},
	}
	revokeCACmd = &cobra.Command{
		Use:         "revoke",
		Short:       "Revoke the identity's CA certificate (creates backup)",
		RunE:        cmdRevokeCA,
		Annotations: map[string]string{"type": "setup"},
	}
	revokePeerCACmd = &cobra.Command{
		Use:         "revoke-peer [service] [revoked cert path]",
		Short:       "Revoke a peer identity's CA certificate and add to local revocation database",
		Args:        cobra.MaximumNArgs(2),
		RunE:        cmdRevokePeerCA,
		Annotations: map[string]string{"type": "setup"},
	}

	newCACfg struct {
		CA identity.CASetupConfig
	}

	getIDCfg struct {
		CA identity.PeerCAConfig
	}

	caExtCfg struct {
		CA identity.FullCAConfig
	}

	revokeCACfg struct {
		CA identity.FullCAConfig
		// TODO: add "broadcast" option to send revocation to network nodes
	}

	revokePeerCACfg struct {
		CA              identity.FullCAConfig
		PeerCA          identity.PeerCAConfig
		RevocationDBURL string
	}
)

func init() {
	// NB: init functions are executed in lexicographical order of filename
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfigDir = confDirParam
	}

	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", defaultConfigDir, "service config directory")
	rootCmd.PersistentFlags().StringVar(&identityDir, "identity-dir", defaultIdentityDir, "root directory for identity output")

	rootCmd.AddCommand(caCmd)

	caCmd.AddCommand(newCACmd)
	caCmd.AddCommand(getIDCmd)
	caCmd.AddCommand(caExtCmd)
	caCmd.AddCommand(revokeCACmd)
	caCmd.AddCommand(revokePeerCACmd)

	process.Bind(newCACmd, &newCACfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(getIDCmd, &getIDCfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(caExtCmd, &caExtCfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(revokeCACmd, &revokeCACfg, defaults, cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(revokePeerCACmd, &revokePeerCACfg, defaults, cfgstruct.ConfDir(defaultConfigDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdNewCA(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	_, err := newCACfg.CA.Create(ctx, os.Stdout)
	return err
}

func cmdGetID(cmd *cobra.Command, args []string) (err error) {
	p, err := getIDCfg.CA.Load()
	if err != nil {
		return err
	}

	fmt.Printf("base58-check node ID:\t%s\n", p.ID)
	fmt.Printf("hex node ID:\t\t%x\n", p.ID)
	fmt.Printf("node ID bytes:\t\t%v\n", p.ID[:])

	difficulty, err := p.ID.Difficulty()
	if err != nil {
		return nil
	}
	fmt.Printf("difficulty:\t\t%d\n", difficulty)
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

	if err := ca.Revoke(); err != nil {
		return err
	}

	updateCfg := identity.FullCAConfig{
		CertPath: revokeCACfg.CA.CertPath,
	}
	if err := updateCfg.Save(ca); err != nil {
		return err
	}
	return nil
}

func cmdRevokePeerCA(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	if len(args) > 0 {
		revokePeerCACfg.CA = identity.FullCAConfig{
			CertPath: filepath.Join(identityDir, args[0], "ca.cert"),
			KeyPath:  filepath.Join(identityDir, args[0], "ca.key"),
		}

		revokePeerCACfg.RevocationDBURL = "bolt://" + filepath.Join(configDir, args[0], "revocations.db")
	}
	if len(args) > 1 {
		revokePeerCACfg.PeerCA = identity.PeerCAConfig{
			CertPath: args[1],
		}
	}

	ca, err := revokePeerCACfg.CA.Load()
	if err != nil {
		return err
	}

	peerCA, err := revokePeerCACfg.PeerCA.Load()
	if err != nil {
		return err
	}

	ext, err := extensions.NewRevocationExt(ca.Key, peerCA.Cert)
	if err != nil {
		return err
	}

	revDB, err := revocation.NewDB(revokePeerCACfg.RevocationDBURL)
	if err != nil {
		return err
	}

	if err = revDB.Put(ctx, []*x509.Certificate{ca.Cert, peerCA.Cert}, ext); err != nil {
		return err
	}
	return nil
}

func cmdCAExtensions(cmd *cobra.Command, args []string) (err error) {
	if len(args) > 0 {
		caExtCfg.CA = identity.FullCAConfig{
			CertPath: filepath.Join(identityDir, args[0], "ca.cert"),
			KeyPath:  filepath.Join(identityDir, args[0], "ca.key"),
		}
	}

	ca, err := caExtCfg.CA.Load()
	if err != nil {
		return err
	}

	return printExtensions(ca.Cert.Raw, ca.Cert.Extensions)
}
