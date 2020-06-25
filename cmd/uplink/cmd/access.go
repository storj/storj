// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink"
)

var (
	inspectCfg AccessConfig
	listCfg    AccessConfig
)

func init() {
	// We skip the use of addCmd here because we only want the configuration options listed
	// above, and addCmd adds a whole lot more than we want.
	accessCmd := &cobra.Command{
		Use:   "access",
		Short: "Set of commands to manage access.",
	}

	inspectCmd := &cobra.Command{
		Use:   "inspect [ACCESS]",
		Short: "Inspect allows you to explode a serialized access into it's constituent parts.",
		RunE:  accessInspect,
		Args:  cobra.MaximumNArgs(1),
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Prints name and associated satellite of all available accesses.",
		RunE:  accessList,
		Args:  cobra.NoArgs,
	}

	RootCmd.AddCommand(accessCmd)
	accessCmd.AddCommand(inspectCmd)
	accessCmd.AddCommand(listCmd)

	process.Bind(inspectCmd, &inspectCfg, defaults, cfgstruct.ConfDir(getConfDir()))
	process.Bind(listCmd, &listCfg, defaults, cfgstruct.ConfDir(getConfDir()))
}

func accessList(cmd *cobra.Command, args []string) (err error) {
	accesses := listCfg.Accesses
	fmt.Println("=========== ACCESSES LIST: name / satellite ================================")
	for name, data := range accesses {
		satelliteAddr, _, _, err := parseAccess(data)
		if err != nil {
			return err
		}

		fmt.Println(name, "/", satelliteAddr)
	}
	return nil
}

func accessInspect(cmd *cobra.Command, args []string) (err error) {
	var access *uplink.Access
	if len(args) == 0 {
		access, err = inspectCfg.GetAccess()
		if err != nil {
			return err
		}
	} else {
		firstArg := args[0]

		access, err = inspectCfg.GetNamedAccess(firstArg)
		if err != nil {
			return err
		}

		if access == nil {
			if access, err = uplink.ParseAccess(firstArg); err != nil {
				return err
			}
		}
	}

	serializedAccesss, err := access.Serialize()
	if err != nil {
		return err
	}

	satAddr, apiKey, ea, err := parseAccess(serializedAccesss)
	if err != nil {
		return err
	}

	fmt.Println("=========== ACCESS INFO ==================================================================")
	fmt.Println("Satellite        :", satAddr)
	fmt.Println("API Key          :", apiKey)
	fmt.Println("Encryption Access:", ea)
	return nil
}

func parseAccess(access string) (sa string, apiKey string, ea string, err error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return "", "", "", errors.New("invalid access grant format")
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return "", "", "", err
	}

	eaData, err := pb.Marshal(p.EncryptionAccess)
	if err != nil {
		return "", "", "", errs.New("unable to marshal encryption access: %v", err)
	}

	apiKey = base58.CheckEncode(p.ApiKey, 0)
	ea = base58.CheckEncode(eaData, 0)
	return p.SatelliteAddr, apiKey, ea, nil
}
