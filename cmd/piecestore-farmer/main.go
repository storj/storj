// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"os"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli"
	"golang.org/x/net/context"

	"storj.io/storj/cmd/piecestore-farmer/commands"
	"storj.io/storj/pkg/process"
)

func main() { process.Must(process.Main(process.ServiceFunc(run))) }

func run(ctx context.Context) error {
	app := cli.NewApp()

	app.Name = "Piece Store Farmer CLI"
	app.Usage = "Connect your drive to the network"
	app.Version = "1.0.0"

	// Flags
	app.Flags = []cli.Flag{}

	app.Commands = []cli.Command{
		commands.Create,
		commands.Start,
		commands.Delete,
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	return app.Run(append([]string{os.Args[0]}, flag.Args()...))
}
