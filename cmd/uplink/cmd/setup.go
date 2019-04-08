// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	setupCfg    UplinkFlags
	confDir     string
	identityDir string
	isDev       bool
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "uplink")
	cfgstruct.SetupFlag(zap.L(), RootCmd, &confDir, "config-dir", defaultConfDir, "main directory for uplink configuration")
	cfgstruct.SetupFlag(zap.L(), RootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for uplink identity credentials")
	cfgstruct.DevFlag(RootCmd, &isDev, false, "use development and test configuration settings")
	RootCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
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

	var override map[string]interface{}
	if setupCfg.Interactive {
		wizard := func() error {
			if !terminal.IsTerminal(0) || !terminal.IsTerminal(1) {
				return fmt.Errorf("stdin/stdout should be terminal")
			}

			// TODO handle signals
			oldState, err := terminal.MakeRaw(0)
			if err != nil {
				return err
			}
			defer terminal.Restore(0, oldState)

			screen := struct {
				io.Reader
				io.Writer
			}{os.Stdin, os.Stdout}
			terminalIn := terminal.NewTerminal(screen, "")

			terminalIn.SetPrompt("Enter your Satellite address: ")
			satelliteAddress, err := terminalIn.ReadLine()
			if err != nil {
				return err
			}

			// TODO add better validation
			if satelliteAddress == "" {
				return errs.New("API key cannot be empty")
			}

			terminalIn.SetPrompt("Enter your API key: ")
			apiKey, err := terminalIn.ReadLine()
			if err != nil {
				return err
			}

			if apiKey == "" {
				return errs.New("API key cannot be empty")
			}

			encKey, err := terminalIn.ReadPassword("Enter your encryption passphrase: ")
			if err != nil {
				return err
			}
			repeatedEncKey, err := terminalIn.ReadPassword("Enter your encryption passphrase again: ")
			if err != nil {
				return err
			}

			if encKey != repeatedEncKey {
				return errs.New("encryption passphrases doesn't match")
			}

			if encKey == "" {
				fmt.Println("Encryption passphare is empty!")
			}

			override = map[string]interface{}{
				"satellite-addr": satelliteAddress,
				"api-key":        apiKey,
				"enc.key":        encKey,
			}
			return nil
		}
		if err := wizard(); err != nil {
			return err
		}
	}

	return process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), override)
}
