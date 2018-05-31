// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"net"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage/redis"
	"google.golang.org/grpc/credentials"
	"storj.io/storj/pkg/utils"
)

var (
	redisAddress  string
	redisPassword string
	db            int
	tlsCertPath   string
	tlsKeyPath    string
  tlsHosts      string
	tlsCreate     bool
	tlsOverwrite  bool
)

func init() {
	flag.StringVar(&redisAddress, "cache", "", "The <IP:PORT> string to use for connection to a redis cache")
	flag.StringVar(&redisPassword, "password", "", "The password used for authentication to a secured redis instance")
	flag.IntVar(&db, "db", 0, "The network cache database")
	flag.StringVar(&tlsCertPath, "tlsCertPath", "", "TLS Certificate file")
	flag.StringVar(&tlsKeyPath, "tlsKeyPath", "", "TLS Key file")
  flag.StringVar(&tlsHosts, "tlsHosts", "", "TLS Key file")
	flag.BoolVar(&tlsCreate, "tlsCreate", false, "If true, generate a new TLS cert/key files")
	flag.BoolVar(&tlsOverwrite, "tlsOverwrite", false, "If true, overwrite existing TLS cert/key files")
}

// NewServer creates a new Overlay Service Server
func NewServer() (*grpc.Server, error) {
	t := &utils.TlsFileOptions{
		CertRelPath: tlsCertPath,
		KeyRelPath:  tlsKeyPath,
		Create:      tlsCreate,
		Overwrite:   tlsOverwrite,
		Hosts:       tlsHosts,
	}

	creds, err := utils.NewServerTLSFromFile(t); if err != nil {
		return nil, err
	}

	credsOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(credsOption)
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

	return grpcServer, nil
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (proto.OverlayClient, error) {
  t := &utils.TlsFileOptions{
    CertRelPath: tlsCertPath,
    KeyRelPath:  tlsKeyPath,
    Create:      tlsCreate,
    Overwrite:   tlsOverwrite,
    Hosts:       tlsHosts,
  }

	creds, err := credentials.NewClientTLSFromFile(t, "")
	if err != nil {
		return nil, err
	}

	credsOption := grpc.WithTransportCredentials(creds)
	opts = append(opts, credsOption)
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return proto.NewOverlayClient(conn), nil
}

// Service contains all methods needed to implement the process.Service interface
type Service struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

// Process is the main function that executes the service
func (s *Service) Process(ctx context.Context) error {
	// bootstrap network
	kad := kademlia.Kademlia{}

	kad.Bootstrap(ctx)
	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, kad)
	if err != nil {
		s.logger.Error("Failed to create a new overlay client", zap.Error(err))
		return err
	}
	if err := cache.Bootstrap(ctx); err != nil {
		s.logger.Error("Failed to boostrap cache", zap.Error(err))
		return err
	}

	// send off cache refreshes concurrently
	go cache.Refresh(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		s.logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

	defer grpcServer.GracefulStop()
	return grpcServer.Serve(lis)

}

// SetLogger adds the initialized logger to the Service
func (s *Service) SetLogger(l *zap.Logger) error {
	s.logger = l
	return nil
}

// SetMetricHandler adds the initialized metric handler to the Service
func (s *Service) SetMetricHandler(m *monkit.Registry) error {
	s.metrics = m
	return nil
}

// InstanceID implements Service.InstanceID
func (s *Service) InstanceID() string { return "" }
