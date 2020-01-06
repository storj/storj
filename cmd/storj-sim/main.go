// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

// Flags contains different flags for commands
type Flags struct {
	Directory string
	Host      string

	SatelliteCount   int
	StorageNodeCount int
	Identities       int

	IsDev bool

	OnlyEnv bool // only do things necessary for loading env vars

	// Connection string for the postgres database to use for storj-sim processes
	Postgres string
	Redis    string

	// Value of first redis db
	RedisStartDB int
}

var printCommands bool

func main() {
	cobra.EnableCommandSorting = false

	var flags Flags

	rootCmd := &cobra.Command{
		Use:   "storj-sim",
		Short: "Storj Network Simulator",
	}

	defaultConfigDir := fpath.ApplicationDir("storj", "local-network")

	configDir := defaultConfigDir
	if os.Getenv("STORJ_NETWORK_DIR") != "" {
		configDir = os.Getenv("STORJ_NETWORK_DIR")
	}

	rootCmd.PersistentFlags().StringVarP(&flags.Directory, "config-dir", "", configDir, "base project directory")
	rootCmd.PersistentFlags().StringVarP(&flags.Host, "host", "", "127.0.0.1", "host to use for network")

	rootCmd.PersistentFlags().IntVarP(&flags.SatelliteCount, "satellites", "", 1, "number of satellites to start")
	rootCmd.PersistentFlags().IntVarP(&flags.StorageNodeCount, "storage-nodes", "", 10, "number of storage nodes to start")
	rootCmd.PersistentFlags().IntVarP(&flags.Identities, "identities", "", 10, "number of identities to create")

	rootCmd.PersistentFlags().BoolVarP(&printCommands, "print-commands", "x", false, "print commands as they are run")
	rootCmd.PersistentFlags().BoolVarP(&flags.IsDev, "dev", "", false, "use configuration values tuned for development")

	rootCmd.PersistentFlags().StringVarP(&flags.Postgres, "postgres", "", os.Getenv("STORJ_SIM_POSTGRES"), "connection string for postgres (defaults to STORJ_SIM_POSTGRES)")
	rootCmd.PersistentFlags().StringVarP(&flags.Redis, "redis", "", os.Getenv("STORJ_SIM_REDIS"), "connection string for redis e.g. 127.0.0.1:6379 (defaults to STORJ_SIM_REDIS)")
	rootCmd.PersistentFlags().IntVarP(&flags.RedisStartDB, "redis-startdb", "", 0, "value of first redis db (defaults to 0)")

	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "local network for testing",
	}

	networkCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run network",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkExec(&flags, args, "run")
			},
		}, &cobra.Command{
			Use:   "env [name]",
			Short: "print environment variables",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkEnv(&flags, args)
			},
		}, &cobra.Command{
			Use:   "setup",
			Short: "setup network",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkExec(&flags, args, "setup")
			},
		}, &cobra.Command{
			Use:   "test <command>",
			Short: "run command with an actual network",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkTest(&flags, args[0], args[1:])
			},
		}, &cobra.Command{
			Use:   "destroy",
			Short: "destroys network if it exists",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkDestroy(&flags, args)
			},
		},
	)

	rootCmd.AddCommand(
		networkCmd,
	)
	rootCmd.SilenceUsage = true
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
