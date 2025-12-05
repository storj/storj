// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/process"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/storagenodedb"
)

type nodeInfoCfg struct {
	storagenode.Config

	JSON bool `default:"false" help:"print node info in JSON format"`
}

func newNodeInfoCmd(f *Factory) *cobra.Command {
	var cfg nodeInfoCfg

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Print storage node info",
		Long: `Print storage node info.

--json should be specified to print output in JSON format.
It is expected that the JSON output will mostly be piped to 'multinode add -'.

WARNING: The output includes the api secret of the storagenode.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdInfo(cmd, &cfg)
		},
		Example: `
#=> print node info
$ storagenode info --config-dir '<path/to/config-dir>' --identity-dir '<path/to/identity-dir>'

#=> print output in JSON format
$ storagenode info --json --config-dir '<path/to/config-dir>' --identity-dir '<path/to/identity-dir>'

#=> add node to multinode dashboard
$ storagenode info --json --config-dir '<path/to/config-dir>' --identity-dir '<path/to/identity-dir>' | multinode add -
`,
		Args: cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func cmdInfo(cmd *cobra.Command, cfg *nodeInfoCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	// TODO(clement): add support for getting info for all available storagenodes

	identity, err := cfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", identity.ID))
	}

	db, err := storagenodedb.OpenExisting(ctx, zap.L().Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("error starting master database on storage node: %v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	service := apikeys.NewService(db.APIKeys())

	apiKey, err := service.Issue(ctx)
	if err != nil {
		return errs.New("error while trying to issue new api key: %v", err)
	}

	if cfg.JSON {
		node := nodes.Node{
			ID:            identity.ID,
			APISecret:     apiKey.Secret,
			PublicAddress: cfg.Contact.ExternalAddress,
		}

		data, err := json.Marshal(node)
		if err != nil {
			return err
		}

		fmt.Println(string(data))
		return nil
	}

	fmt.Printf(`
ID: %s
API Secret: %s
Public Address: %s
`, identity.ID, apiKey.Secret, cfg.Contact.ExternalAddress)

	return nil
}
