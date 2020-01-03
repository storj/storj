// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc"
	"storj.io/storj/certificate/certificateclient"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
)

const (
	defaultSignerAddress = "certs.alpha.storj.io:8888"
)

var (
	rootCmd = &cobra.Command{
		Use:   "identity",
		Short: "Identity management",
	}

	newServiceCmd = &cobra.Command{
		Use:         "create <service>",
		Short:       "Create a new full identity for a service",
		Args:        cobra.ExactArgs(1),
		RunE:        cmdNewService,
		Annotations: map[string]string{"type": "setup"},
	}

	authorizeCmd = &cobra.Command{
		Use:         "authorize <service> <auth-token>",
		Short:       "Send a certificate signing request for a service's CA certificate",
		Args:        cobra.ExactArgs(2),
		RunE:        cmdAuthorize,
		Annotations: map[string]string{"type": "setup"},
	}

	//nolint
	config struct {
		Difficulty     uint64 `default:"36" help:"minimum difficulty for identity generation"`
		Concurrency    uint   `default:"4" help:"number of concurrent workers for certificate authority generation"`
		ParentCertPath string `help:"path to the parent authority's certificate chain"`
		ParentKeyPath  string `help:"path to the parent authority's private key"`
		Signer         certificateclient.Config
		// TODO: ideally the default is the latest version; can't interpolate struct tags
		IdentityVersion uint `default:"0" help:"identity version to use when creating an identity or CA"`

		Version checker.Config
	}

	identityDir, configDir string
	defaultIdentityDir     = fpath.ApplicationDir("storj", "identity")
	defaultConfigDir       = fpath.ApplicationDir("storj", "identity")
)

func init() {
	rootCmd.AddCommand(newServiceCmd)
	rootCmd.AddCommand(authorizeCmd)

	process.Bind(newServiceCmd, &config, defaults, cfgstruct.ConfDir(defaultConfigDir), cfgstruct.IdentityDir(defaultIdentityDir))
	process.Bind(authorizeCmd, &config, defaults, cfgstruct.ConfDir(defaultConfigDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func main() {
	process.Exec(rootCmd)
}

func serviceDirectory(serviceName string) string {
	return filepath.Join(identityDir, serviceName)
}

func cmdNewService(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	err := checker.CheckProcessVersion(ctx, zap.L(), config.Version, version.Build, "Identity")
	if err != nil {
		return err
	}

	serviceDir := serviceDirectory(args[0])

	caCertPath := filepath.Join(serviceDir, "ca.cert")
	caKeyPath := filepath.Join(serviceDir, "ca.key")
	identCertPath := filepath.Join(serviceDir, "identity.cert")
	identKeyPath := filepath.Join(serviceDir, "identity.key")

	caConfig := identity.CASetupConfig{
		CertPath:       caCertPath,
		KeyPath:        caKeyPath,
		Difficulty:     config.Difficulty,
		Concurrency:    config.Concurrency,
		ParentCertPath: config.ParentCertPath,
		ParentKeyPath:  config.ParentKeyPath,
		VersionNumber:  config.IdentityVersion,
	}

	status, err := caConfig.Status()
	if err != nil {
		return err
	}
	if status != identity.NoCertNoKey {
		return errs.New("CA certificate and/or key already exists, NOT overwriting!")
	}

	identConfig := identity.SetupConfig{
		CertPath: identCertPath,
		KeyPath:  identKeyPath,
	}

	status, err = identConfig.Status()
	if err != nil {
		return err
	}
	if status != identity.NoCertNoKey {
		return errs.New("Identity certificate and/or key already exists, NOT overwriting!")
	}

	ca, caerr := caConfig.Create(ctx, os.Stdout)
	if caerr != nil {
		return caerr
	}

	_, iderr := identConfig.Create(ca)
	if iderr != nil {
		return iderr
	}

	fmt.Printf("Unsigned identity is located in %q\n", serviceDir)
	fmt.Println("Please *move* CA key to secure storage - it is only needed for identity management!")
	fmt.Printf("\t%s\n", caConfig.KeyPath)
	return nil
}

func cmdAuthorize(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	err = checker.CheckProcessVersion(ctx, zap.L(), config.Version, version.Build, "Identity")
	if err != nil {
		return err
	}

	serviceDir := serviceDirectory(args[0])

	authToken := args[1]

	caCertPath := filepath.Join(serviceDir, "ca.cert")
	caConfig := identity.PeerCAConfig{
		CertPath: caCertPath,
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

	if config.Signer.Address == "" {
		config.Signer.Address = defaultSignerAddress
	}

	// Ensure we dont enforce a signed Peer Identity
	config.Signer.TLS.UsePeerCAWhitelist = false

	revocationDB, err := revocation.NewDBFromCfg(config.Signer.TLS)
	if err != nil {
		return errs.New("error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	tlsOptions, err := tlsopts.NewOptions(ident, config.Signer.TLS, nil)
	if err != nil {
		return err
	}

	client, err := certificateclient.New(ctx, rpc.NewDefaultDialer(tlsOptions), config.Signer.Address)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	signedChainBytes, err := client.Sign(ctx, authToken)
	if err != nil {
		return err
	}

	signedChain, err := pkcrypto.CertsFromDER(signedChainBytes)
	if err != nil {
		return nil
	}

	err = caConfig.SaveBackup(ca)
	if err != nil {
		return err
	}

	// NB: signedChain is this identity's CA + signer chain.
	ca.Cert = signedChain[0]
	ca.RestChain = signedChain[1:]
	err = caConfig.Save(ca)
	if err != nil {
		return err
	}

	err = identConfig.PeerConfig().SaveBackup(ident.PeerIdentity())
	if err != nil {
		return err
	}

	ident.RestChain = signedChain[1:]
	ident.CA = ca.Cert
	err = identConfig.PeerConfig().Save(ident.PeerIdentity())
	if err != nil {
		return err
	}

	fmt.Println("Identity successfully authorized using single use authorization token.")
	fmt.Printf("Please back-up \"%s\" to a safe location.\n", serviceDir)
	return nil
}

func printExtensions(cert []byte, exts []pkix.Extension) error {
	hash := pkcrypto.SHA256Hash(cert)
	b64Hash, err := json.Marshal(hash)
	if err != nil {
		return err
	}
	fmt.Printf("Cert hash: %s\n", b64Hash)
	fmt.Println("Extensions:")
	for _, e := range exts {
		var data interface{}
		if e.Id.Equal(extensions.RevocationExtID) {
			var rev extensions.Revocation
			if err := rev.Unmarshal(e.Value); err != nil {
				return err
			}
			data = rev
		} else {
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
