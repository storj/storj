// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/urfave/cli"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pstore"
)

var ArgError = errs.Class("argError")

func main() {
	app := cli.NewApp()

	app.Name = "Piece Store CLI"
	app.Usage = "Store data in hash folder structure"
	app.Version = "1.0.0"

	app.Flags = []cli.Flag{}

	app.Commands = []cli.Command{
		{
			Name:      "store",
			Aliases:   []string{"s"},
			Usage:     "Store data by hash",
			ArgsUsage: "[hash] [dataPath] [storeDir]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("Missing data Hash")
				}

				if c.Args().Get(1) == "" {
					return ArgError.New("No input file specified")
				}

				if c.Args().Get(2) == "" {
					return ArgError.New("No output directory specified")
				}

				file, err := os.Open(c.Args().Get(1))
				if err != nil {
					return err
				}
				// Close the file when we are done
				defer file.Close()

				fileInfo, err := os.Stat(c.Args().Get(1))

				if fileInfo.IsDir() {
					return ArgError.New(fmt.Sprintf("Path (%s) is a directory, not a file", c.Args().Get(1)))
				}

				err = pstore.Store(c.Args().Get(0), file, int64(fileInfo.Size()), 0, c.Args().Get(2))

				return err
			},
		},
		{
			Name:      "retrieve",
			Aliases:   []string{"r"},
			Usage:     "Retrieve data by hash and print to Stdout",
			ArgsUsage: "[hash] [storeDir]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("Missing data Hash")
				}
				if c.Args().Get(1) == "" {
					return ArgError.New("Missing file path")
				}
				fileInfo, err := os.Stat(c.Args().Get(1))

				if os.IsNotExist(err) {
					return err
				}

				if fileInfo.IsDir() != true {
					return ArgError.New(fmt.Sprintf("Path (%s) is a file, not a directory", c.Args().Get(1)))
				}

				err = pstore.Retrieve(c.Args().Get(0), os.Stdout, -1, 0, c.Args().Get(1))

				return err
			},
		},
		{
			Name:      "delete",
			Aliases:   []string{"d"},
			Usage:     "Delete data by hash",
			ArgsUsage: "[hash] [storeDir]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("Missing data Hash")
				}
				if c.Args().Get(1) == "" {
					return ArgError.New("No directory specified")
				}
				err := pstore.Delete(c.Args().Get(0), c.Args().Get(1))

				return err
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
