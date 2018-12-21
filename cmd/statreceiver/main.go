// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"storj.io/storj/internal/fpath"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

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

	cfgstruct.Bind(cmd.Flags(), &Config, cfgstruct.ConfDir(defaultConfDir))
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
				log.Printf("Failed to close input: %s", err)
			}
		}()

		input = inputFile
	}

	s := luacfg.NewScope()
	err := errs.Combine(
		s.RegisterVal("deliver", Deliver),
		s.RegisterVal("filein", NewFileSource),
		s.RegisterVal("fileout", NewFileDest),
		s.RegisterVal("udpin", NewUDPSource),
		s.RegisterVal("udpout", NewUDPDest),
		s.RegisterVal("parse", NewParser),
		s.RegisterVal("print", NewPrinter),
		s.RegisterVal("pcopy", NewPacketCopier),
		s.RegisterVal("mcopy", NewMetricCopier),
		s.RegisterVal("pbuf", NewPacketBuffer),
		s.RegisterVal("mbuf", NewMetricBuffer),
		s.RegisterVal("packetfilter", NewPacketFilter),
		s.RegisterVal("appfilter", NewApplicationFilter),
		s.RegisterVal("instfilter", NewInstanceFilter),
		s.RegisterVal("keyfilter", NewKeyFilter),
		s.RegisterVal("sanitize", NewSanitizer),
		s.RegisterVal("graphite", NewGraphiteDest),
		s.RegisterVal("db", NewDBDest),
		s.RegisterVal("pbufprep", NewPacketBufPrep),
		s.RegisterVal("mbufprep", NewMetricBufPrep),
	)
	if err != nil {
		return err
	}

	err = s.Run(input)
	if err != nil {
		return err
	}

	select {}
}
