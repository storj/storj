// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package pointerdb

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/pointerdb"
)

type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

func (s *Service) Process(ctx context.Context, _ *cobra.Command, _ []string) (err error) {
	grpcServer := grpc.NewServer()

	proto.RegisterPointerDBServer(grpcServer, pointerdb.NewServer(bdb, s.logger))
	s.logger.Debug(fmt.Sprintf("server listening on port %d", *port))

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)
}

func (s *Service) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

// func setEnv() error {
// 	viper.SetEnvPrefix("api")
// 	viper.BindEnv("key")
// 	viper.AutomaticEnv()
// 	return nil
// }

func (s *Service) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

func (s *Service) InstanceID() string { return "" }
