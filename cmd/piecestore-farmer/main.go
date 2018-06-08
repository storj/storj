// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"sort"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mr-tron/base58/base58"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	"storj.io/storj/pkg/process"
	proto "storj.io/storj/protos/overlay"
	pb "storj.io/storj/protos/piecestore"
)

func newID() string {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	encoding := base58.Encode(b)

	return encoding[:20]
}

func connectToKad(id, ip, kadlistenport, kadaddress string) *kademlia.Kademlia {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadaddress,
		},
	}

	kad, err := kademlia.NewKademlia([]proto.Node{node}, ip, kadlistenport)
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

func main() { process.Must(process.Main(process.ServiceFunc(run))) }

func run(ctx context.Context) error {
	app := cli.NewApp()

	app.Name = "Piece Store Farmer CLI"
	app.Usage = "Connect your drive to the network"
	app.Version = "1.0.0"

	// Flags
	app.Flags = []cli.Flag{}
	var kadhost string
	var kadport string
	var kadlistenport string
	var pshost string
	var psport string
	var dir string

	app.Commands = []cli.Command{
		{
			Name:      "create",
			Aliases:   []string{"c"},
			Usage:     "create farmer node",
			ArgsUsage: "",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "pieceStoreHost", Usage: "Farmer's public ip/host", Destination: &pshost},
				cli.StringFlag{Name: "pieceStorePort", Usage: "`port` where piece store data is accessed", Destination: &psport},
				cli.StringFlag{Name: "kademliaPort", Usage: "Kademlia server `host`", Destination: &kadport},
				cli.StringFlag{Name: "kademliaHost", Usage: "Kademlia server `host`", Destination: &kadhost},
				cli.StringFlag{Name: "kademliaListenPort", Usage: "Kademlia server `host`", Destination: &kadlistenport},
				cli.StringFlag{Name: "dir", Usage: "`dir` of drive being shared", Destination: &dir},
			},
			Action: func(c *cli.Context) error {
				nodeID := newID()

				usr, err := user.Current()
				if err != nil {
					return err
				}

				viper.SetDefault("piecestore.host", "127.0.0.1")
				viper.SetDefault("piecestore.port", "7777")
				viper.SetDefault("piecestore.dir", usr.HomeDir)
				viper.SetDefault("piecestore.id", nodeID)
				viper.SetDefault("kademlia.host", "bootstrap.storj.io")
				viper.SetDefault("kademlia.port", "8080")
				viper.SetDefault("kademlia.listen.port", "7776")

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

				// Create empty file at configPath
				_, err = os.Create(fullPath)
				if err != nil {
					return err
				}

				if pshost != "" {
					viper.Set("piecestore.host", pshost)
				}
				if psport != "" {
					viper.Set("piecestore.port", psport)
				}
				if dir != "" {
					viper.Set("piecestore.dir", dir)
				}
				if kadhost != "" {
					viper.Set("kademlia.host", kadhost)
				}
				if kadport != "" {
					viper.Set("kademlia.port", kadport)
				}
				if kadlistenport != "" {
					viper.Set("kademlia.listen.port", kadlistenport)
				}

				if err := viper.WriteConfig(); err != nil {
					return err
				}

				path := viper.ConfigFileUsed()

				fmt.Printf("Config: %s\n", path)
				fmt.Printf("ID: %s\n", nodeID)

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

				nodeid := viper.GetString("piecestore.id")
				pshost = viper.GetString("piecestore.host")
				psport = viper.GetString("piecestore.port")
				kadlistenport = viper.GetString("kademlia.listen.port")
				kadport = viper.GetString("kademlia.port")
				kadhost = viper.GetString("kademlia.host")
				piecestoreDir := viper.GetString("piecestore.dir")
				dbPath := path.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeid), "/ttl-data.db")
				dataDir := path.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeid), "/piece-store-data/")

				if err = os.MkdirAll(piecestoreDir, 0700); err != nil {
					log.Fatalf(err.Error())
				}

				_ = connectToKad(nodeid, pshost, kadlistenport, fmt.Sprintf("%s:%s", kadhost, kadport))

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
				lis, err := net.Listen("tcp", fmt.Sprintf(":%s", psport))
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

	return app.Run(append([]string{os.Args[0]}, flag.Args()...))
}
