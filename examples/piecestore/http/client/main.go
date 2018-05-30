// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
	"github.com/zeebo/errs"

	"storj.io/storj/examples/piecestore/http/client/downloader"
	"storj.io/storj/examples/piecestore/http/client/uploader"
	"storj.io/storj/examples/piecestore/http/client/utils"
)

var argError = errs.Class("argError")

func main() {
	app := cli.NewApp()
	app.Name = "storj-client"
	app.Usage = ""
	app.Version = "1.0.0"

	app.Flags = []cli.Flag{}

	app.Commands = []cli.Command{
		{
			Name:      "upload",
			Aliases:   []string{"u"},
			Usage:     "Upload data",
			ArgsUsage: "[path]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("No path provided")
				}

				err := uploader.PrepareUpload(c.Args().Get(0))
				if err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name:      "download",
			Aliases:   []string{"d"},
			Usage:     "Download data",
			ArgsUsage: "[hash] [path]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("No hash provided")
				}

				if c.Args().Get(1) == "" {
					return argError.New("No path provided")
				}

				err := downloader.PrepareDownload(c.Args().Get(0), c.Args().Get(1))
				if err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name:    "list-files",
			Aliases: []string{"l"},
			Usage:   "List all files",
			Action: func(c *cli.Context) error {
				err := utils.ListFiles()
				if err != nil {
					return err
				}

				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
