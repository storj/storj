// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"github.com/zeebo/structs"

	"storj.io/common/cfgstruct"
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
)

// newExecCmd creates a new exec command.
func newExecCmd(ctx context.Context, ball *mud.Ball, factory *Factory, selector mud.ComponentSelector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "execute selected components (VERY, VERY, EXPERIMENTAL)",
		RunE: func(cmd *cobra.Command, args []string) error {
			vip := viper.New()
			if err := vip.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			prefix := os.Getenv("STORJ_ENV_PREFIX")
			if prefix == "" {
				prefix = "storj"
			}

			vip.SetEnvPrefix(prefix)
			vip.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
			err := LoadConfig(cmd, vip)
			allSettings := vip.AllSettings()
			if err != nil {
				return err
			}

			err = mud.ForEachDependency(ball, selector, func(c *mud.Component) error {
				cfg, ok := mud.GetTagOf[config.Config](c)
				if ok {
					structs.Decode(allSettings[cfg.Prefix], c.Instance())
				}
				return nil
			}, mud.Tagged[config.Config]())
			if err != nil {
				return err
			}
			err = cmdExec(ctx, ball, selector)
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		},
	}

	err := config.BindAll(context.Background(), cmd, ball, selector, factory.Defaults, cfgstruct.ConfDir(factory.ConfDir), cfgstruct.IdentityDir(factory.IdentityDir))
	if err != nil {
		panic(err)
	}

	return cmd
}

func cmdExec(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) (err error) {
	err = modular.Initialize(ctx, ball, selector)
	if err != nil {
		return err
	}
	err1 := modular.Run(ctx, ball, selector)
	err2 := modular.Close(ctx, ball, selector)
	return errs.Combine(err1, err2)

}

// LoadConfig loads configuration into *viper.Viper from file specified with "config-dir" flag.
func LoadConfig(cmd *cobra.Command, vip *viper.Viper) error {
	cfgFlag := cmd.Flags().Lookup("config-dir")
	if cfgFlag != nil && cfgFlag.Value.String() != "" {
		path := filepath.Join(os.ExpandEnv(cfgFlag.Value.String()), "config.yaml")
		if fileExists(path) {
			setupCommand := cmd.Annotations["type"] == "setup"
			vip.SetConfigFile(path)
			if err := vip.ReadInConfig(); err != nil && !setupCommand {
				return err
			}
		}
	}
	return nil
}

// fileExists checks whether file exists, handle error correctly if it doesn't.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("failed to check for file existence: %v", err)
	}
	return true
}
