// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/utils"
)

// ID is the type used to specify a ID key in the process context
type ID string

// Service defines the interface contract for all Storj services
type Service interface {
	Process(context.Context) error
	SetLogger(*zap.Logger) error
	SetMetricHandler(*monkit.Registry) error
	GetServer() *grpc.Server
}

var (
	id ID = "SrvID"
)

// Serve initializes a new Service
func Serve(s Service) error {
	flag.Parse()
	ctx := context.Background()
	uid := uuid.New().String()

	logger, err := utils.NewLogger("", zap.Fields(zap.String("SrvID", uid)))
	if err != nil {
		return err
	}
	defer logger.Sync()

	ctx, cf := context.WithCancel(context.WithValue(ctx, id, uid))
	defer cf()

	s.SetLogger(logger)
	s.SetMetricHandler(monkit.NewRegistry())
	s.Process(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	ss := s.GetServer()
	// Start gRPC server
	go ss.Serve(lis)
	defer ss.GracefulStop()

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan

	logger.Info("Failed to initialize TCP connection", zap.Any("sig", sig))

	return nil
}
