// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "payments",
		Short: "generate a payments report for storage nodes on your network",
		RunE:  cmdPayments,
	}

	configDir string
	id        identity.Config
	database  string
	test      bool
)

func init() {
	cobra.OnInitialize(readConfig)
	rootCmd.Flags().StringVar(&configDir, "config-dir", fpath.ApplicationDir("storj", "satellite"), "path to satellite config directory")
	rootCmd.Flags().BoolVarP(&test, "test", "t", false, "generate a payment report using default directory for storj-sim satellite")
}

func readConfig() {
	if test {
		configDir = fpath.ApplicationDir("storj", "local-network", "satellite", "0")
	}
	viper.AddConfigPath(configDir)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	database = viper.GetString("database")
	id.CertPath = viper.GetString("identity.cert-path")
	id.KeyPath = viper.GetString("identity.key-path")
}

func cmdPayments(cmd *cobra.Command, args []string) error {
	fmt.Println("entering payments generatecsv")

	ctx := process.Ctx(cmd)

	layout := "2006-01-02"
	start, err := time.Parse(layout, args[0])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}
	end, err := time.Parse(layout, args[1])
	if err != nil {
		return errs.New("Invalid date format. Please use YYYY-MM-DD")
	}

	// Ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	id, err := id.Load()
	if err != nil {
		return err
	}

	report, err := generateCSV(ctx, configDir, database, id.ID.String(), start, end)
	if err != nil {
		return err
	}

	fmt.Println("Created payments report at", report)
	return nil
}

func main() {
	process.Exec(rootCmd)
}
