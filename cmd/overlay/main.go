// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/storage/redis"
)

// SrvID is the unique operational identifier for the service
type SrvID string

// ReqID is the unique identifier for each request that the overlay service handles
type ReqID string

var (
	redisAddress  string
	redisPassword string
	db            int
	env           string
	id            SrvID = "ID"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

// Main configures and runs an overlay node
func Main() error {
	app := cli.NewApp()

	app.Name = "Overlay Network Server"
	app.Usage = "Initializes a node on the overlay Network"
	app.Version = "1.0.0"
	app.Action = serve

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "cache",
			Value:       "",
			Usage:       "The <IP:PORT> string to use for connection to a redis cache",
			Destination: &redisAddress,
		},
		cli.StringFlag{
			Name:        "password",
			Value:       "",
			Usage:       "The password used for authentication to a secured redis instance",
			Destination: &redisPassword,
		},
		cli.IntFlag{
			Name:        "db",
			Value:       1,
			Usage:       "The network cache database",
			Destination: &db,
		},
		cli.StringFlag{
			Name:        "env",
			Value:       "dev",
			Usage:       "Specifies the environment this server will run in",
			Destination: &env,
		},
	}

	return app.Run(os.Args)
}

func serve(c *cli.Context) error {
	uid := uuid.New().String()
	logger, err := newLogger(env, zap.Fields(zap.String("SrvID", uid)))
	if err != nil {
		return err
	}
	defer logger.Sync()
	// TODO(coyle): metrics
	ctx := context.Background()

	ctx, cf := context.WithCancel(context.WithValue(ctx, id, uid))
	defer cf()

	// bootstrap network
	kad := kademlia.Kademlia{}

	kad.Bootstrap(ctx)
	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, kad)
	if err != nil {
		logger.Error("Failed to create a new overlay client", zap.Error(err))
		return err
	}
	if err := cache.Bootstrap(ctx); err != nil {
		logger.Error("Failed to boostrap cache", zap.Error(err))
		return err
	}

	// send off cache refreshes concurrently
	go cache.Refresh(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	s := overlay.NewServer()
	// Start gRPC server
	go s.Serve(lis)
	defer s.GracefulStop()

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan

	logger.Info("Failed to initialize TCP connection", zap.Any("sig", sig))

	return nil
}

func newLogger(e string, options ...zap.Option) (*zap.Logger, error) {
	switch strings.ToLower(e) {
	case "dev", "development":
		return zap.NewDevelopment(options...)
	case "prod", "production":
		return zap.NewProduction(options...)
	}

	return zap.NewNop(), nil
}
