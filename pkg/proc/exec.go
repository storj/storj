// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package proc

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"storj.io/storj/pkg/utils"
)

func defaultConfigPath(name string) string {
	home, err := homedir.Dir()
	if err != nil {
		return ""
	}
	if name == "" {
		name = filepath.Base(os.Args[0])
	}
	return filepath.Join(home, ".storj", fmt.Sprintf("%s.json", name))
}

// Execute runs a *cobra.Command and sets up Storj-wide process configuration
// like a configuration file and logging.
// TODO: add back metrics
func Execute(cmd *cobra.Command) {
	flag.String("log.disp", "prod",
		"switch to 'dev' to get more output")
	cfgFile := flag.String("config", defaultConfigPath(cmd.Name()),
		"config file")

	var outer_defers []func()
	defer func() {
		for _, fn := range outer_defers {
			fn()
		}
	}()

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	cobra.OnInitialize(func() {
		viper.BindPFlags(cmd.Flags())
		viper.SetEnvPrefix("storj")
		viper.AutomaticEnv()
		if *cfgFile != "" {
			viper.SetConfigFile(*cfgFile)
			viper.ReadInConfig()
		}

		logger, err := utils.NewLogger(viper.GetString("log.disp"))
		if err != nil {
			panic(err)
		}
		outer_defers = append(outer_defers,
			zap.RedirectStdLog(logger),
			zap.ReplaceGlobals(logger),
			func() { logger.Sync() },
		)
	})

	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
