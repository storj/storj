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

func (s *Service) Process(ctx context.Context) error {
	fmt.Printf("starting netstate process %+v\n", s)

	if err := setEnv(); err != nil {
		s.logger.Error("error configuring environment for netstate server")
		return err
	}

	bdb, err := boltdb.NewClient(s.logger, *dbPath, boltdb.PointerBucket)

	if err != nil {
		return err
	}
	defer bdb.Close()

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()

	proto.RegisterNetStateServer(grpcServer, NewServer(bdb, s.logger))
	s.logger.Debug(fmt.Sprintf("server listening on port %d", *port))

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)
}

type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

func (s *Service) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

func setEnv() error {
	viper.SetEnvPrefix("API")
	viper.AutomaticEnv()
	return nil
}

func (s *Service) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

func (s *Service) InstanceID() string { return "" }

// func main() {
// 	setEnv()
// 	process.Must(process.Main(&serv{}))
// }
