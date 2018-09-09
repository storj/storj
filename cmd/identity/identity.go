// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/provider"
)

var (
	idCmd = &cobra.Command{
		Use:   "id",
		Short: "Manage identities",
	}

	newIDCmd = &cobra.Command{
		Use:   "new",
		Short: "Creates a new identity from an existing certificate authority",
		RunE:  cmdNewID,
	}

	newIDCfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentitySetupConfig
	}
)

func init() {
	rootCmd.AddCommand(idCmd)
	idCmd.AddCommand(newIDCmd)
	cfgstruct.Bind(newIDCmd.Flags(), &newIDCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdNewID(cmd *cobra.Command, args []string) (err error) {
	ca, err := newIDCfg.CA.Load()
	if err != nil {
		return err
	}

	s := newIDCfg.Identity.Stat()
	if s == provider.NoCertNoKey || newIDCfg.Identity.Overwrite {
		_, err := newIDCfg.Identity.Create(ca)
		return err
	}
	return provider.ErrSetup.New("identity file(s) exist: %s", s)
}
