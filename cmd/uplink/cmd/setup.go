// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/internal/wizard"
	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/uplink/setup"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	setupCfg UplinkFlags
)

func init() {
	RootCmd.AddCommand(setupCmd)
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.SetupMode())
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	// Ensure use the default port if the user only specifies a host.
	err = ApplyDefaultHostAndPortToAddrFlag(cmd, "satellite-addr")
	if err != nil {
		return err
	}

	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("uplink configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	// override is required because the default value of Enc.KeyFilepath is ""
	// and setting the value directly in setupCfg.Enc.KeyFiletpathon will set the
	// value in the config file but commented out.
	usedEncryptionKeyFilepath := setupCfg.Enc.KeyFilepath
	if usedEncryptionKeyFilepath == "" {
		usedEncryptionKeyFilepath = filepath.Join(setupDir, ".encryption.key")
	}

	if setupCfg.NonInteractive {
		return cmdSetupNonInteractive(cmd, setupDir, usedEncryptionKeyFilepath)
	}

	return cmdSetupInteractive(cmd, setupDir, usedEncryptionKeyFilepath)
}

// cmdSetupNonInteractive sets up uplink non-interactively.
//
// encryptionKeyFilepath should be set to the filepath indicated by the user or
// or to a default path whose directory tree exists.
func cmdSetupNonInteractive(cmd *cobra.Command, setupDir string, encryptionKeyFilepath string) error {
	if setupCfg.Enc.EncryptionKey != "" {
		err := setup.SaveEncryptionKey(setupCfg.Enc.EncryptionKey, encryptionKeyFilepath)
		if err != nil {
			return err
		}
	}

	override := map[string]interface{}{
		"enc.key-filepath": encryptionKeyFilepath,
	}

	err := process.SaveConfigWithAllDefaults(
		cmd.Flags(), filepath.Join(setupDir, process.DefaultCfgFilename), override)
	if err != nil {
		return err
	}

	if setupCfg.Enc.EncryptionKey != "" {
		_, _ = fmt.Printf("Your encryption key is saved to: %s\n", encryptionKeyFilepath)
	}

	return nil
}

// cmdSetupInteractive sets up uplink interactively.
//
// encryptionKeyFilepath should be set to the filepath indicated by the user or
// or to a default path whose directory tree exists.
func cmdSetupInteractive(cmd *cobra.Command, setupDir string, encryptionKeyFilepath string) error {
	ctx := process.Ctx(cmd)

	satelliteAddress, err := wizard.PromptForSatellite(cmd)
	if err != nil {
		return err
	}

	apiKeyString, err := wizard.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	apiKey, err := libuplink.ParseAPIKey(apiKeyString)
	if err != nil {
		return Error.Wrap(err)
	}

	passphrase, err := wizard.PromptForEncryptionPassphrase()
	if err != nil {
		return Error.Wrap(err)
	}

	uplk, err := setupCfg.NewUplink(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, uplk.Close()) }()

	project, err := uplk.OpenProject(ctx, satelliteAddress, apiKey)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	key, err := project.SaltedKeyFromPassphrase(ctx, passphrase)
	if err != nil {
		return Error.Wrap(err)
	}

	err = setup.SaveEncryptionKey(string(key[:]), encryptionKeyFilepath)
	if err != nil {
		return err
	}

	var override = map[string]interface{}{
		"api-key":          apiKeyString,
		"satellite-addr":   satelliteAddress,
		"enc.key-filepath": encryptionKeyFilepath,
	}

	err = process.SaveConfigWithAllDefaults(
		cmd.Flags(), filepath.Join(setupDir, process.DefaultCfgFilename), override)
	if err != nil {
		return nil
	}

	// if there is an error with this we cannot do that much and the setup process
	// has ended OK, so we ignore it.
	_, _ = fmt.Printf(`
Your encryption key is saved to: %s

Your Uplink CLI is configured and ready to use!

Some things to try next:

* Run 'uplink --help' to see the operations that can be performed

* See https://github.com/storj/docs/blob/master/Uplink-CLI.md#usage for some example commands
	`, encryptionKeyFilepath)

	return nil
}

// ApplyDefaultHostAndPortToAddrFlag applies the default host and/or port if either is missing in the specified flag name.
func ApplyDefaultHostAndPortToAddrFlag(cmd *cobra.Command, flagName string) error {
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		// No flag found for us to handle.
		return nil
	}

	address, err := ApplyDefaultHostAndPortToAddr(flag.Value.String(), flag.DefValue)
	if err != nil {
		return Error.Wrap(err)
	}

	if flag.Value.String() == address {
		// Don't trip the flag set bit
		return nil
	}

	return Error.Wrap(flag.Value.Set(address))
}

// ApplyDefaultHostAndPortToAddr applies the default host and/or port if either is missing in the specified address.
func ApplyDefaultHostAndPortToAddr(address, defaultAddress string) (string, error) {
	defaultHost, defaultPort, err := net.SplitHostPort(defaultAddress)
	if err != nil {
		return "", Error.Wrap(err)
	}

	addressParts := strings.Split(address, ":")
	numberOfParts := len(addressParts)

	if numberOfParts > 1 && len(addressParts[0]) > 0 && len(addressParts[1]) > 0 {
		// address is host:port so skip applying any defaults.
		return address, nil
	}

	// We are missing a host:port part. Figure out which part we are missing.
	indexOfPortSeparator := strings.Index(address, ":")
	lengthOfFirstPart := len(addressParts[0])

	if indexOfPortSeparator < 0 {
		if lengthOfFirstPart == 0 {
			// address is blank.
			return defaultAddress, nil
		}
		// address is host
		return net.JoinHostPort(addressParts[0], defaultPort), nil
	}

	if indexOfPortSeparator == 0 {
		// address is :1234
		return net.JoinHostPort(defaultHost, addressParts[1]), nil
	}

	// address is host:
	return net.JoinHostPort(addressParts[0], defaultPort), nil
}
