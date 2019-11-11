// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "create SATELLITE APIKEY ENCPASS",
		Short: "Create a scope",
		RunE:  scopeCreateMain,
	}, scopeCmd)
}

// scopeCreateMain is the function executed when scopeCreateCmd is called.
func scopeCreateMain(cmd *cobra.Command, args []string) (err error) {
	var (
		satelliteAddress string
		apiKeyString     string
		passphrase       string

		apiKey libuplink.APIKey
	)

	switch len(args) {
	case 3:
		// Parse satellite address.
		vip, err := process.Viper(cmd)
		if err != nil {
			return err
		}

		satelliteAddress = args[0]
		satelliteAddress, err = ApplyDefaultHostAndPortToAddr(satelliteAddress, vip.GetString("satellite-addr"))
		if err != nil {
			return Error.Wrap(err)
		}

		// Parse API key.
		apiKeyString = args[1]
		apiKey, err = libuplink.ParseAPIKey(apiKeyString)
		if err != nil {
			return Error.Wrap(err)
		}

		// Parse encryption passphrase.
		passphrase = args[2]
	default:
		return fmt.Errorf("Invalid number of arguments")
	}

	ctx, _ := process.Ctx(cmd)

	project, err := cfg.GetProject(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, project.Close()) }()

	key, err := project.SaltedKeyFromPassphrase(ctx, passphrase)
	if err != nil {
		return Error.Wrap(err)
	}

	scopeData, err := (&libuplink.Scope{
		SatelliteAddr:    satelliteAddress,
		APIKey:           apiKey,
		EncryptionAccess: libuplink.NewEncryptionAccessWithDefaultKey(*key),
	}).Serialize()
	if err != nil {
		return Error.Wrap(err)
	}

	fmt.Println(scopeData)

	return nil
}
