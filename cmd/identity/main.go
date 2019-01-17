// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "identity",
		Short: "Identity management",
	}

	newServiceCmd = &cobra.Command{
		Use:   "new <service>[, ...]",
		Short: "Create a new full identity for one or more services",
		Args:  cobra.MinimumNArgs(1),
		RunE:  cmdNewService,
	}

	csrCmd = &cobra.Command{
		Use:   "csr <service>",
		Short: "Send a certificate signing request for a service's CA certificate",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdCSR,
	}

	//nolint
	config struct {
		BaseDir        string `default:"$CONFDIR/identity" help:"Directory containing service subdirectories"`
		Difficulty     uint64 `default:"15" help:"minimum difficulty for identity generation"`
		Timeout        string `default:"5m" help:"timeout for CA generation; golang duration string (0 no timeout)"`
		Concurrency    uint   `default:"4" help:"number of concurrent workers for certificate authority generation"`
		ParentCertPath string `help:"path to the parent authority's certificate chain"`
		ParentKeyPath  string `help:"path to the parent authority's private key"`
		Signer         certificates.CertClientConfig
	}

	defaultConfDir = fpath.ApplicationDir("storj", "identity")
)

func init() {
	rootCmd.AddCommand(newServiceCmd)
	rootCmd.AddCommand(csrCmd)

	cfgstruct.Bind(newServiceCmd.Flags(), &config, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(csrCmd.Flags(), &config, cfgstruct.ConfDir(defaultConfDir))
}

func main() {
	process.Exec(rootCmd)
}

func cmdNewService(cmd *cobra.Command, args []string) error {
	serviceErrs := new(errs.Group)
	for _, service := range args {
		serviceDir := filepath.Join(config.BaseDir, service)
		caCertPath := filepath.Join(serviceDir, "ca.cert")
		caKeyPath := filepath.Join(serviceDir, "ca.key")
		identCertPath := filepath.Join(serviceDir, "identity.cert")
		identKeyPath := filepath.Join(serviceDir, "identity.key")

		ca, err := identity.CASetupConfig{
			CertPath: caCertPath,
			KeyPath:  caKeyPath,
		}.Create(process.Ctx(cmd))
		if err != nil {
			serviceErrs.Add(err)
		}

		_, err = identity.SetupConfig{
			CertPath: identCertPath,
			KeyPath:  identKeyPath,
		}.Create(ca)
		if err != nil {
			serviceErrs.Add(err)
		}
	}

	return serviceErrs.Err()
}

func cmdCSR(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	serviceDir := filepath.Join(config.BaseDir, args[0])
	caCertPath := filepath.Join(serviceDir, "ca.cert")
	caKeyPath := filepath.Join(serviceDir, "ca.key")
	caConfig := identity.FullCAConfig{
		CertPath: caCertPath,
		KeyPath:  caKeyPath,
	}
	identCertPath := filepath.Join(serviceDir, "identity.cert")
	identKeyPath := filepath.Join(serviceDir, "identity.key")
	identConfig := identity.Config{
		CertPath: identCertPath,
		KeyPath:  identKeyPath,
	}

	ca, err := caConfig.Load()
	if err != nil {
		return err
	}
	ident, err := identConfig.Load()
	if err != nil {
		return err
	}

	signedChainBytes, err := config.Signer.Sign(ctx, ident)
	if err != nil {
		return errs.New("error occurred while signing certificate: %s\n(identity files were still generated and saved, if you try again existing files will be loaded)", err)
	}

	signedChain, err := identity.ParseCertChain(signedChainBytes)
	if err != nil {
		return nil
	}

	err = caConfig.SaveBackup(ca)
	if err != nil {
		return err
	}

	ca.Cert = signedChain[0]
	ca.RestChain = signedChain[1:]
	err = identity.FullCAConfig{
		CertPath: caConfig.CertPath,
	}.Save(ca)
	if err != nil {
		return err
	}

	err = identConfig.SaveBackup(ident)
	if err != nil {
		return err
	}

	ident.RestChain = signedChain[1:]
	ident.CA = ca.Cert
	err = identity.Config{
		CertPath: identConfig.CertPath,
	}.Save(ident)
	if err != nil {
		return err
	}
	return nil
}

func printExtensions(cert []byte, exts []pkix.Extension) error {
	hash, err := peertls.SHA256Hash(cert)
	if err != nil {
		return err
	}
	b64Hash, err := json.Marshal(hash)
	if err != nil {
		return err
	}
	fmt.Printf("Cert hash: %s\n", b64Hash)
	fmt.Println("Extensions:")
	for _, e := range exts {
		var data interface{}
		switch e.Id.String() {
		case peertls.ExtensionIDs[peertls.RevocationExtID].String():
			var rev peertls.Revocation
			if err := rev.Unmarshal(e.Value); err != nil {
				return err
			}
			data = rev
		default:
			data = e.Value
		}
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("\t%s: %s\n", e.Id, out)
	}
	return nil
}
