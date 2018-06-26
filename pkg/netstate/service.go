// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	proto "storj.io/storj/protos/netstate"
	"storj.io/storj/storage/boltdb"
)

var (
	port   = flag.Int("port", 8080, "port")
	dbPath = flag.String("netstateDB", "netstate.db", "netstate db path")
)

// Process fits the `Process` interface for services
func (s *Service) Process(ctx context.Context) {
	if err := setEnv(); err != nil {
		s.errors <- err
	}

	bdb, err := boltdb.NewClient(s.logger, *dbPath, boltdb.PointerBucket)

	if err != nil {
		s.errors <- err
	}
	defer bdb.Close()

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		s.errors <- err
	}

	grpcServer := grpc.NewServer()

	proto.RegisterNetStateServer(grpcServer, NewServer(bdb, s.logger))
	s.logger.Debug(fmt.Sprintf("server listening on port %d", *port))

	defer grpcServer.GracefulStop()
	err = grpcServer.Serve(lis)
	if err != nil {
		s.errors <- err
	}
}

// NewService creates a new Service pointer for Netstate service
func NewService(l *zap.Logger, m *monkit.Registry) *Service {
	return &Service{
		errors:  make(chan error),
		metrics: m,
		logger:  l,
	}
}

// Service struct for process
type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
	errors  chan error
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

// Errors returns error channel for process
func (s *Service) Errors() chan error {
	return s.errors
}

// InstanceID assigns a new instance ID to the process
func (s *Service) InstanceID() string { return "" }
