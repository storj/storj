// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	RootCmd = &cobra.Command{
		Use:   "id",
		Short: "Identity",
	}
	newCmd = &cobra.Command{
		Use:   "new",
		Short: "Creates a new identity from an existing certificate authority",
		RunE:  cmdNew,
	}

	newCfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentitySetupConfig
	}
)

func init() {
	RootCmd.AddCommand(newCmd)
	cfgstruct.Bind(newCmd.Flags(), &newCfg)
}

func cmdNew(cmd *cobra.Command, args []string) (err error) {
	ca, err := newCfg.CA.Load()
	if err != nil {
		return err
	}

	if s := newCfg.Identity.Stat(); s == provider.NoCertNoKey || newCfg.Identity.Overwrite {
		_, err := newCfg.Identity.Create(ca)
		if err != nil {
			return err
		}
		return nil
	} else {
		return provider.ErrSetup.New("identity file(s) exist: %s", s)
	}

	return nil
}

func main() {
	process.Exec(RootCmd)
}
