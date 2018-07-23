// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	proto "storj.io/storj/protos/pointerdb"
	"storj.io/storj/storage/boltdb"
)

var (
	port   = flag.Int("port", 8080, "port")
	dbPath = flag.String("pointerdb", "pointerdb.db", "pointerdb db path")
)

// Process fits the `Process` interface for services
func (s *Service) Process(ctx context.Context, _ *cobra.Command, _ []string) error {
	if err := setEnv(); err != nil {
		return err
	}

	bdb, err := boltdb.NewClient(s.logger, *dbPath, boltdb.PointerBucket)
	if err != nil {
		return err
	}
	defer func() {
		if err := bdb.Close(); err != nil {
			s.logger.Error("failed to close boltDB client", zap.Error(err))
		}
	}()

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()

	proto.RegisterPointerDBServer(grpcServer, NewServer(bdb, s.logger))
	s.logger.Debug(fmt.Sprintf("server listening on port %d", *port))

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)
}

// Service struct for process
type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

// SetLogger for process
func (s *Service) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

func setEnv() error {
	viper.SetEnvPrefix("api")
	err := viper.BindEnv("key")
	if err != nil {
		log.Println("Failed to set API Creds: ", err)
		return err
	}
	viper.AutomaticEnv()
	return nil
}

// SetMetricHandler for  process
func (s *Service) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

// InstanceID assigns a new instance ID to the process
func (s *Service) InstanceID() string { return "" }
