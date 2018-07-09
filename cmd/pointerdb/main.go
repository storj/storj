// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/netstate"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	proto "storj.io/storj/protos/pointerdb"
)

var (
	port   = flag.Int("port", 8080, "port")
	dbPath = flag.String("db", "pointers.db", "db path")
)

func main() {
	err := process.Main(process.ConfigEnvironment, &netstate.Service{})
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()

	proto.RegisterPointerDBServer(grpcServer, pointerdb.NewServer(bdb, s.logger))
	s.logger.Debug(fmt.Sprintf("server listening on port %d", *port))

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)
}

type serv struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

func (s *serv) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

func setEnv() error {
	viper.SetEnvPrefix("api")
	viper.BindEnv("key")
	viper.AutomaticEnv()
	return nil
}

func (s *serv) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

func (s *serv) InstanceID() string { return "" }

func main() {
	setEnv()
	process.Must(process.Main(&serv{}))
}
