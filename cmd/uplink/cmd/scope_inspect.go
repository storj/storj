// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	libuplink "storj.io/storj/lib/uplink"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "inspect SCOPE",
		Short: "Dump information about a scope",
		RunE:  scopeInspectMain,
	}, scopeCmd)
}

// scopeInspectMain is the function executed when scopeInspectCmd is called.
func scopeInspectMain(cmd *cobra.Command, args []string) (err error) {
	var (
		scopeb58 string
		scope    *libuplink.Scope
	)

	switch len(args) {
	case 1:
		scopeb58 = args[0]

		// Parse scope string.
		scope, err = libuplink.ParseScope(scopeb58)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid number of arguments")
	}

	// FIXME: This is non-ideal. It would have been nice to be able to just
	// marshal the scope directly but APIKey and EncryptionAccess do not
	// expose any fields. Unfortunately there also isn't a way exposed to
	// get the caveats or restrictions on the scope (as would be found from
	// one generated through the `share` command).
	type Scope struct {
		SatelliteAddr    string `json:"satellite_address"`
		APIKey           string `json:"api_key"`
		EncryptionAccess string `json:"encryption_access"`
	}

	encAccessString, err := scope.EncryptionAccess.Serialize()
	if err != nil {
		return err
	}

	s := &Scope{
		SatelliteAddr:    scope.SatelliteAddr,
		APIKey:           scope.APIKey.Serialize(),
		EncryptionAccess: encAccessString,
	}

	bs, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(bs))

	return nil
}
