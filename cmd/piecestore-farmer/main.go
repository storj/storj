// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

func newID() (string) {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	encoding := base64.URLEncoding.EncodeToString(b)

	return encoding[:20]
}

func connectToKad(id, ip, port string) *kademlia.Kademlia {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "bootstrap.storj.io:8080",
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
				nodeID := newID()

				usr, err := user.Current()
				if err != nil {
					return err
				}

				viper.SetDefault("ip", "127.0.0.1")
				viper.SetDefault("port", "7777")
				viper.SetDefault("rootdir", path.Join(usr.HomeDir, nodeID))

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
						return errors.New("config already exists")
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
					viper.Set("rootdir", path.Join(dir, nodeID))
				}

				viper.Set("datadir", path.Join(viper.GetString("rootdir"), "/piece-store-data/"))
				viper.Set("ttl", path.Join(viper.GetString("rootdir"), "/ttl-data.db"))

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
				if c.Args().Get(0) == "" {
					return errors.New("no id specified")
				}

				usr, err := user.Current()
				if err != nil {
					log.Fatalf(err.Error())
				}

				configPath := path.Join(usr.HomeDir, ".storj/")
				viper.AddConfigPath(configPath)
				viper.SetConfigName(c.Args().Get(0))
				viper.SetConfigType("yaml")
				if err := viper.ReadInConfig(); err != nil {
					log.Fatalf(err.Error())
				}

				nodeid := viper.GetString("nodeid")
				ip := viper.GetString("ip")
				port := viper.GetString("port")
				piecestoreDir := viper.GetString("rootdir")
				dataDir := viper.GetString("datadir")
				dbPath := viper.GetString("ttl")

				if err = os.MkdirAll(piecestoreDir, 0700); err != nil {
					fmt.Println("I failed")
					log.Fatalf(err.Error())
				}

				_ = connectToKad(nodeid, ip, port)

				fileInfo, err := os.Stat(piecestoreDir)
				if err != nil {
					log.Fatalf(err.Error())
				}
				if fileInfo.IsDir() != true {
					log.Fatalf("Error: %s is not a directory", piecestoreDir)
				}

				ttlDB, err := ttl.NewTTL(dbPath)
				if err != nil {
					log.Fatalf("failed to open DB")
				}

				// create a listener on TCP port
				lis, err := net.Listen("tcp", fmt.Sprintf(":%s", viper.GetString("port")))
				if err != nil {
					log.Fatalf("failed to listen: %v", err)
				}
				defer lis.Close()

				// create a server instance
				s := server.Server{PieceStoreDir: dataDir, DB: ttlDB}

				// create a gRPC server object
				grpcServer := grpc.NewServer()

				// attach the api service to the server
				pb.RegisterPieceStoreRoutesServer(grpcServer, &s)

				// routinely check DB and delete expired entries
				go func() {
					err := s.DB.DBCleanup(dataDir)
					log.Fatalf("Error in DBCleanup: %v", err)
				}()

				// start the server
				if err := grpcServer.Serve(lis); err != nil {
					log.Fatalf("failed to serve: %s", err)
				}
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
