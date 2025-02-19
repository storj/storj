// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/structs"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// LoadConfig loads configuration with vip, and binds it to the registered (and selected) Config components.
// Simplified version of the logic, what we already have in storj.io/shared/process.
func LoadConfig(cmd *cobra.Command, ball *mud.Ball, selector mud.ComponentSelector) (err error) {

	vip := viper.New()

	if err := vip.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	prefix := os.Getenv("STORJ_ENV_PREFIX")
	if prefix == "" {
		prefix = "STORJ"
	}

	vip.SetEnvPrefix(prefix)
	vip.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	err = readConfigFile(cmd, vip)
	if err != nil {
		return err
	}
	vip.AutomaticEnv()

	allSettings := vip.AllSettings()

	err = mud.ForEachDependency(ball, selector, func(c *mud.Component) error {
		cfg, ok := mud.GetTagOf[config.Config](c)
		if ok {

			var settings map[string]interface{}

			if cfg.Prefix == "" {
				settings = allSettings
			} else {
				settings = allSettings
				for _, sub := range strings.Split(cfg.Prefix, ".") {
					settings = settings[sub].(map[string]interface{})
					if settings == nil {
						break
					}
				}
			}
			if c.Instance() != nil && settings != nil {
				res := structs.Decode(settings, c.Instance())
				if res.Error != nil {
					return res.Error
				}
			}
		}
		return nil
	}, mud.Tagged[config.Config]())
	if err != nil {
		return err
	}
	return nil
}

// readConfigFile loads configuration into *viper.Viper from file specified with "config-dir" flag.
func readConfigFile(cmd *cobra.Command, vip *viper.Viper) error {
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
