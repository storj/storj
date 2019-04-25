// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

var (
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	setupCfg UplinkFlags
	confDir  string
	defaults cfgstruct.BindOpt

	// Error is the default uplink setup errs class
	Error = errs.Class("uplink setup error")
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	cfgstruct.SetupFlag(zap.L(), RootCmd, &confDir, "config-dir", defaultConfDir, "main directory for uplink configuration")
	defaults = cfgstruct.DefaultsFlag(RootCmd)
	RootCmd.AddCommand(setupCmd)
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, defaults, cfgstruct.ConfDir(confDir))
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

	var (
		encryptionKeyFilepath = filepath.Join(setupDir, ".encryption.key")
		encryptionKey         []byte
		override              = map[string]interface{}{
			"enc.key-filepath": encryptionKeyFilepath,
		}
	)
	if !setupCfg.NonInteractive {
		_, err = fmt.Print(`
Pick satellite to use:
  [1] mars.tardigrade.io
  [2] jupiter.tardigrade.io
  [3] saturn.tardigrade.io
Please enter numeric choice or enter satellite address manually [1]: `)
		if err != nil {
			return err
		}
		satellites := []string{"mars.tardigrade.io", "jupiter.tardigrade.io", "saturn.tardigrade.io"}
		var satelliteAddress string
		n, err := fmt.Scanln(&satelliteAddress)
		if err != nil {
			if n == 0 {
				// fmt.Scanln cannot handle empty input
				satelliteAddress = satellites[0]
			} else {
				return err
			}
		}

		// TODO add better validation
		if satelliteAddress == "" {
			return errs.New("satellite address cannot be empty")
		} else if len(satelliteAddress) == 1 {
			switch satelliteAddress {
			case "1":
				satelliteAddress = satellites[0]
			case "2":
				satelliteAddress = satellites[1]
			case "3":
				satelliteAddress = satellites[2]
			default:
				return errs.New("Satellite address cannot be one character")
			}
		}

		satelliteAddress, err = ApplyDefaultHostAndPortToAddr(satelliteAddress, cmd.Flags().Lookup("satellite-addr").Value.String())
		if err != nil {
			return err
		}
		override["satellite-addr"] = satelliteAddress

		_, err = fmt.Print("Enter your API key: ")
		if err != nil {
			return err
		}
		var apiKey string
		n, err = fmt.Scanln(&apiKey)
		if err != nil && n != 0 {
			return err
		}

		if apiKey == "" {
			return errs.New("API key cannot be empty")
		}
		override["api-key"] = apiKey

		_, err = fmt.Print("Enter your encryption passphrase: ")
		if err != nil {
			return err
		}
		encryptionKey, err = terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		_, err = fmt.Println()
		if err != nil {
			return err
		}

		_, err = fmt.Print("Enter your encryption passphrase again: ")
		if err != nil {
			return err
		}
		repeatedEncKey, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		_, err = fmt.Println()
		if err != nil {
			return err
		}

		if !bytes.Equal(encryptionKey, repeatedEncKey) {
			return errs.New("encryption passphrases doesn't match")
		}

		if len(encryptionKey) == 0 {
			_, err = fmt.Println("Warning: Encryption passphrase is empty!")
			if err != nil {
				return err
			}
		} else {
			key := storj.NewKey(encryptionKey)
			err = SaveEncryptionKey(key, encryptionKeyFilepath)
			if err != nil {
				return err
			}
		}

		err = process.SaveConfigWithAllDefaults(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), override)
		if err != nil {
			return nil
		}

		_, err = fmt.Println(`
Your Uplink CLI is configured and ready to use!

Some things to try next:

* Run 'uplink --help' to see the operations that can be performed

* See https://github.com/storj/docs/blob/master/Uplink-CLI.md#usage for some example commands
		`)
		if err != nil {
			return nil
		}

		return nil
	}

	return process.SaveConfigWithAllDefaults(
		cmd.Flags(), filepath.Join(setupDir, process.DefaultConfFilename), nil)
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

// SaveEncryptionKey saves the key in a new file which will be stored in
// filepath.
//
// key is a pointer for avoiding having several copies of the key in memory.
//
// It returns an error if the key is nil, the directory doesn't exist, the
// file already exists or there is an I/O error.
func SaveEncryptionKey(key *storj.Key, filepath string) (err error) {
	if key == nil {
		return errors.New("key cannot be nil")
	}

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory path doesn't exist. %+v", err)
		}

		if os.IsExist(err) {
			return fmt.Errorf("file key already exists. %+v", err)
		}

		return err
	}

	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	_, err = file.Write(key[:])
	return err
}
