// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
	"storj.io/storj/internal/version/checker"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/process"
	"storj.io/storj/storagenode"
	"strconv"
)

// NB: these vars are intended to be set with -ldflags -X arguments during compilation.
var (
	exitCode string
	version  string

	// NB: using cobra + process package because go flags package doesn't parse flags after first non-flag arg.
	rootCmd = &cobra.Command{
		Short: "fake binary for testing",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "fake run subcommand for testing",
		RunE:  cmdRun,
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "fake version subcommand for testing",
		RunE:  cmdVersion,
	}

	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "fake setup subcommand for testing",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}

	// NB: fake subcommand config flag structs must match real commands' otherwise usage is printed and exit code is 0.
	runCfg struct {
		checker.Config
		Identity identity.Config

		BinaryLocation string `help:"the storage node executable binary location" default:"storagenode.exe"`
		ServiceName    string `help:"storage node OS service name" default:"storagenode"`
		Log string `help:"path to log file, if empty standard output will be used" default:""`

		ConfigDir string `help:"main directory for test configuration" default:""`
	}

	setupCfg struct {
		storagenode.Config

		IdentityDir string `help:"main directory for identity credentials" default:"" setup:"true"`
		ConfigDir string `help:"main directory for test configuration" default:"" setup:"true"`
	}
)

func init() {
	cfgstruct.Bind(runCmd.Flags(), &runCfg)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.SetupMode())

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(setupCmd)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	var code int
	if exitCode == "" {
		code = 0
	} else {
		code, err = strconv.Atoi(exitCode)
		if err != nil {
			panic(err)
		}
	}
	os.Exit(code)
	return nil
}

func cmdVersion(cmd *cobra.Command, args []string) (err error) {
	fmt.Printf("Version: %s\n", version)
	return nil
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	base := filepath.Base(os.Args[0])
	base = base[:len(base)-len(filepath.Ext(base))]
	if base == "storagenode" && setupCfg.ConfigDir != "" {
		configFile, err := os.Create(filepath.Join(setupCfg.ConfigDir, "config.yaml"))
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := configFile.Close(); err != nil {
				log.Fatal(err)
			}
		}()
		if _, err := configFile.Write([]byte("# test config\n")); err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

func main() {
	process.Exec(rootCmd)
}
