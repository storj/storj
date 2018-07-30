// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	proto "storj.io/storj/pkg/statdb/proto"
)

var (
	addr   = flag.String("addr", ":8080", "listen address")
	dbPath = flag.String("statdb", "stats.db", "stats db path")
)

// Process fits the `Process` interface for services
func (s *Service) Process(ctx context.Context, _ *cobra.Command, _ []string) error {
	if err := setEnv(); err != nil {
		return err
	}

	// start grpc server
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()

	ns, err := NewServer("sqlite3", *dbPath, s.logger)
	if err != nil {
		return err
	}
	proto.RegisterStatDBServer(grpcServer, ns)
	s.logger.Debug(fmt.Sprintf("server listening on address %s", *addr))

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
	viper.SetEnvPrefix("API")
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
