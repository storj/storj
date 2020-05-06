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

	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/cmd/internal/wizard"
	"storj.io/uplink"
	"storj.io/uplink/backcomp"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
		Args:        cobra.NoArgs,
	}
	setupCfg UplinkFlags
)

func init() {
	RootCmd.AddCommand(setupCmd)
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.SetupMode())

	// NB: access is not supported by `setup` or `import`
	cfgstruct.SetBoolAnnotation(setupCmd.Flags(), "access", cfgstruct.BasicHelpAnnotationName, false)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	if cmd.Flag("access").Changed {
		return ErrAccessFlag
	}

	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	satelliteAddress, err := wizard.PromptForSatellite(cmd)
	if err != nil {
		return Error.Wrap(err)
	}

	// apply helpful default host and port to the address
	vip, err := process.Viper(cmd)
	if err != nil {
		return err
	}
	satelliteAddress, err = ApplyDefaultHostAndPortToAddr(
		satelliteAddress, vip.GetString("satellite-addr"))
	if err != nil {
		return Error.Wrap(err)
	}

	var (
		accessName                    string
		defaultSerializedAccessExists bool
	)

	setupCfg.AccessConfig = setupCfg.AccessConfig.normalize()
	defaultSerializedAccessExists = IsSerializedAccess(setupCfg.Access)

	accessName, err = wizard.PromptForAccessName()
	if err != nil {
		return Error.Wrap(err)
	}

	if accessName == "default" && defaultSerializedAccessExists {
		return Error.New("a default access already exists")
	}

	if access, err := setupCfg.GetNamedAccess(accessName); err == nil && access != nil {
		return Error.New("an access with the name %q already exists", accessName)
	}

	apiKeyString, err := wizard.PromptForAPIKey()
	if err != nil {
		return Error.Wrap(err)
	}

	passphrase, err := wizard.PromptForEncryptionPassphrase()
	if err != nil {
		return Error.Wrap(err)
	}

	uplinkConfig := uplink.Config{
		DialTimeout: setupCfg.Client.DialTimeout,
	}

	overrides := make(map[string]interface{})
	analyticEnabled, err := wizard.PromptForTracing()
	if err != nil {
		return Error.Wrap(err)
	}
	if analyticEnabled {
		enableTracing(overrides)
	} else {
		// set metrics address to empty string so we can disable it on each operation
		overrides["metrics.addr"] = ""
	}

	ctx, _ := withTelemetry(cmd)

	var access *uplink.Access
	if setupCfg.PBKDFConcurrency == 0 {
		access, err = uplinkConfig.RequestAccessWithPassphrase(ctx, satelliteAddress, apiKeyString, passphrase)
	} else {
		access, err = backcomp.RequestAccessWithPassphraseAndConcurrency(ctx, uplinkConfig, satelliteAddress, apiKeyString, passphrase, uint8(setupCfg.PBKDFConcurrency))
	}
	if err != nil {
		return Error.Wrap(err)
	}
	accessData, err := access.Serialize()
	if err != nil {
		return Error.Wrap(err)
	}

	// NB: accesses should always be `map[string]interface{}` for "conventional"
	// config serialization/flattening.
	accesses := toStringMapE(setupCfg.Accesses)
	accesses[accessName] = accessData
	overrides["accesses"] = accesses

	saveCfgOpts := []process.SaveConfigOption{
		process.SaveConfigWithOverrides(overrides),
		process.SaveConfigRemovingDeprecated(),
	}

	if setupCfg.Access == "" {
		saveCfgOpts = append(saveCfgOpts, process.SaveConfigWithOverride("access", accessName))
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	configPath := filepath.Join(setupDir, process.DefaultCfgFilename)
	err = process.SaveConfig(cmd, configPath, saveCfgOpts...)
	if err != nil {
		return Error.Wrap(err)
	}

	// if there is an error with this we cannot do that much and the setup process
	// has ended OK, so we ignore it.
	fmt.Println(`
Your Uplink CLI is configured and ready to use!

Some things to try next:

* See https://documentation.tardigrade.io/api-reference/uplink-cli for some example commands`)

	return nil
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
