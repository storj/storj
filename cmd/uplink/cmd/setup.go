// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
		UserAgent:   setupCfg.Client.UserAgent,
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
	accesses := convertAccessesForViper(setupCfg.Accesses)
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

* See https://documentation.tardigrade.io/api-reference/uplink-cli for some example commands`)

	return nil
}
