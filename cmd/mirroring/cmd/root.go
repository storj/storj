// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"storj.io/mirroring/cmd/cp"
	"storj.io/mirroring/cmd/get"
	"storj.io/mirroring/cmd/list"
	"storj.io/mirroring/cmd/make_bucket"
	"storj.io/mirroring/cmd/put"
	"storj.io/mirroring/cmd/server"
	"storj.io/mirroring/cmd/version"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "storj.io/mirroring/pkg/gateway"
)

var (
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mirroring",
	Short: "A backup mirroring util",
	Long:  `A backup mirroring util`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	addCommands()

	rootCmd.Execute()
}
func addCommands() {
	rootCmd.AddCommand(make_bucket.Cmd)
	rootCmd.AddCommand(cp.Cmd)
	rootCmd.AddCommand(put.Cmd)
	rootCmd.AddCommand(get.Cmd)
	rootCmd.AddCommand(list.Cmd)
	// rootCmd.AddCommand(delete.Cmd)
	rootCmd.AddCommand(version.Cmd)
	// rootCmd.AddCommand(config.Cmd)
	rootCmd.AddCommand(server.Cmd)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mirroring.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.Set("configPath", cfgFile)
	}
}
