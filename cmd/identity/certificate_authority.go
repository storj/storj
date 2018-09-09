// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	caCmd = &cobra.Command{
		Use:   "ca",
		Short: "Manage certificate authorities",
	}

	newCACmd = &cobra.Command{
		Use:   "new",
		Short: "Create a new certificate authority",
		RunE:  cmdNewCA,
	}
	getIDCmd = &cobra.Command{
		Use:   "id",
		Short: "Get the id of a CA",
		RunE:  cmdGetID,
	}

	newCACfg struct {
		CA provider.CASetupConfig
	}

	getIDCfg struct {
		CA provider.PeerCAConfig
	}
)

func init() {
	rootCmd.AddCommand(caCmd)
	caCmd.AddCommand(newCACmd)
	caCmd.AddCommand(getIDCmd)
	cfgstruct.Bind(newCACmd.Flags(), &newCACfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(getIDCmd.Flags(), &getIDCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdNewCA(cmd *cobra.Command, args []string) error {
	_, err := newCACfg.CA.Create(process.Ctx(cmd))
	return err
}

func cmdGetID(cmd *cobra.Command, args []string) (err error) {
	p, err := getIDCfg.CA.Load()
	if err != nil {
		return err
	}

	fmt.Println(p.ID.String())
	return nil
}
