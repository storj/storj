// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	// "net"
	"os"
	// "path"
  "sort"

	// _ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"
	// "google.golang.org/grpc"
  "github.com/urfave/cli"

	// "storj.io/storj/cmd/piecestore-farmer/config"
	"storj.io/storj/pkg/kademlia"
	// "storj.io/storj/pkg/piecestore/rpc/server"
	// "storj.io/storj/pkg/piecestore/rpc/server/ttl"
	proto "storj.io/storj/protos/overlay"
	// pb "storj.io/storj/protos/piecestore"
)

func connectToKad(id, ip, port string) *kademlia.Kademlia {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "130.211.168.182:4242",
		},
	}

	kad, err := kademlia.NewKademlia([]proto.Node{node}, ip, port)
	if err != nil {
		log.Fatalf("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		log.Fatalf("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(context.Background()); err != nil {
		log.Fatalf("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad
}

func main() {
	app := cli.NewApp()

	app.Name = "Piece Store Farmer CLI"
	app.Usage = "Connect your drive to the network"
	app.Version = "1.0.0"

  // Flags
  app.Flags = []cli.Flag{}
  var port string
  var host string
  var dir string

	app.Commands = []cli.Command{
    {
			Name:      "create",
			Aliases:   []string{"c"},
			Usage:     "create farmer node",
			ArgsUsage: "",
      Flags: []cli.Flag{
        cli.StringFlag{Name: "port, p", Usage: "Run farmer at `port`", Destination: &port},
        cli.StringFlag{Name: "host, n", Usage: "Farmers public `hostname`", Destination: &host},
        cli.StringFlag{Name: "dir, d", Usage: "`dir` of drive being shared", Destination: &dir},
      },
			Action: func(c *cli.Context) error {
        fmt.Println(port)
				return nil
			},
		},
		{
			Name:      "start",
			Aliases:   []string{"s"},
			Usage:     "start farmer node",
			ArgsUsage: "[id]",
			Action: func(c *cli.Context) error {

				return nil
			},
		},
    {
			Name:      "delete",
			Aliases:   []string{"d"},
			Usage:     "delete farmer node",
			ArgsUsage: "[id]",
			Action: func(c *cli.Context) error {

				return nil
			},
		},
    {
			Name:      "list",
			Aliases:   []string{"l"},
			Usage:     "list farmer nodes",
			ArgsUsage: "",
			Action: func(c *cli.Context) error {

				return nil
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
