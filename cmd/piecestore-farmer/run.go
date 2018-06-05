// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"fmt"
	"log"
	// "net"
	"os"
	"os/user"
	"path"
	"path/filepath"
  "sort"

	// _ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"
	// "google.golang.org/grpc"
	"github.com/spf13/viper"
  "github.com/urfave/cli"

	"storj.io/storj/pkg/piecestore"
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
				nodeID := pstore.DetermineID()
				usr, err := user.Current()
				if err != nil {
					return err
				}

				defaultDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
  			if err != nil {
          return err
  			}

				viper.SetDefault("ip", "")
				viper.SetDefault("port", "7777")
				viper.SetDefault("datadir", defaultDir)

				viper.SetConfigName(nodeID)
				viper.SetConfigType("yaml")

				configPath := path.Join(usr.HomeDir, ".storj/")
				if err = os.MkdirAll(configPath, 0700); err != nil {
					return err
				}

				viper.AddConfigPath(configPath)

				fullPath := path.Join(configPath, fmt.Sprintf("%s.yaml", nodeID))
				_, err = os.Stat(fullPath)
				if os.IsExist(err) {
					if err != nil {
						return errors.New("Config already exists!")
					}
					return err
				}

			  // Create emty file at configPath
				_, err = os.Create(fullPath)
				if err != nil {
					return err
				}

				viper.Set("nodeid", nodeID)

				if host != "" {
					viper.Set("ip", host)
				}
				if port != "" {
					viper.Set("port", port)
				}
				if dir != "" {
					viper.Set("datadir", dir)
				}

				if err := viper.WriteConfig(); err != nil {
					return err
				}

				path := viper.ConfigFileUsed()

				fmt.Println(path)
				fmt.Println(nodeID)

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
