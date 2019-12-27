// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/storj/cmd/statreceiver/luacfg"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
)

// Config is the set of configuration values we care about
var Config struct {
	Input string `default:"" help:"path to configuration file"`
}

func main() {
	defaultConfDir := fpath.ApplicationDir("storj", "statreceiver")

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "stat receiving",
		RunE:  Main,
	}
	defaults := cfgstruct.DefaultsFlag(cmd)
	process.Bind(cmd, &Config, defaults, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(cmd)
}

// Main is the real main method
func Main(cmd *cobra.Command, args []string) error {
	var input io.Reader
	switch Config.Input {
	case "":
		return fmt.Errorf("--input path to script is required")
	case "stdin":
		input = os.Stdin
	default:
		inputFile, err := os.Open(Config.Input)
		if err != nil {
			return err
		}
		defer func() {
			if err := inputFile.Close(); err != nil {
				log.Printf("Failed to close input: %scope", err)
			}
		}()

		input = inputFile
	}

	scope := luacfg.NewScope()
	err := errs.Combine(
		scope.RegisterVal("deliver", Deliver),
		scope.RegisterVal("filein", NewFileSource),
		scope.RegisterVal("fileout", NewFileDest),
		scope.RegisterVal("udpin", NewUDPSource),
		scope.RegisterVal("udpout", NewUDPDest),
		scope.RegisterVal("parse", NewParser),
		scope.RegisterVal("print", NewPrinter),
		scope.RegisterVal("pcopy", NewPacketCopier),
		scope.RegisterVal("mcopy", NewMetricCopier),
		scope.RegisterVal("pbuf", NewPacketBuffer),
		scope.RegisterVal("mbuf", NewMetricBuffer),
		scope.RegisterVal("packetfilter", NewPacketFilter),
		scope.RegisterVal("appfilter", NewApplicationFilter),
		scope.RegisterVal("instfilter", NewInstanceFilter),
		scope.RegisterVal("keyfilter", NewKeyFilter),
		scope.RegisterVal("sanitize", NewSanitizer),
		scope.RegisterVal("graphite", NewGraphiteDest),
		scope.RegisterVal("db", NewDBDest),
		scope.RegisterVal("pbufprep", NewPacketBufPrep),
		scope.RegisterVal("mbufprep", NewMetricBufPrep),
	)
	if err != nil {
		return err
	}

	err = scope.Run(input)
	if err != nil {
		return err
	}

	select {}
}
