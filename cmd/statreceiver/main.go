// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/luacfg"
	"storj.io/storj/pkg/process"
)

var Config struct {
	Input string `default:"" help:"path to configuration file"`
}

const defaultConfDir = "$HOME/.storj/statreceiver"

func main() {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "stat receiving",
		RunE:  Main,
	}
	cfgstruct.Bind(cmd.Flags(), &Config, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"),
		"path to configuration")
	process.Exec(cmd)
}

func Main(cmd *cobra.Command, args []string) error {
	var input io.Reader
	switch Config.Input {
	case "":
		return fmt.Errorf("--input path to script is required")
	case "stdin":
		input = os.Stdin
	default:
		fh, err := os.Open(Config.Input)
		if err != nil {
			return err
		}
		defer fh.Close()
		input = fh
	}

	s := luacfg.NewScope()
	s.RegisterVal("deliver", Deliver)
	s.RegisterVal("filein", NewFileSource)
	s.RegisterVal("fileout", NewFileDest)
	s.RegisterVal("udpin", NewUDPSource)
	s.RegisterVal("udpout", NewUDPDest)
	s.RegisterVal("parse", NewParser)
	s.RegisterVal("print", NewPrinter)
	s.RegisterVal("pcopy", NewPacketCopier)
	s.RegisterVal("mcopy", NewMetricCopier)
	s.RegisterVal("pbuf", NewPacketBuffer)
	s.RegisterVal("mbuf", NewMetricBuffer)
	s.RegisterVal("packetfilter", NewPacketFilter)
	s.RegisterVal("appfilter", NewApplicationFilter)
	s.RegisterVal("instfilter", NewInstanceFilter)
	s.RegisterVal("keyfilter", NewKeyFilter)
	s.RegisterVal("sanitize", NewSanitizer)
	s.RegisterVal("graphite", NewGraphiteDest)
	s.RegisterVal("db", NewDBDest)
	s.RegisterVal("pbufprep", NewPacketBufPrep)
	s.RegisterVal("mbufprep", NewMetricBufPrep)
	err := s.Run(input)
	if err != nil {
		return err
	}

	select {}
}
