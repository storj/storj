// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage/redis"
)

var (
	redisAddress, redisPassword, httpPort, bootstrapIP, bootstrapPort, localPort string
	db                                                                           int
	srvPort                                                                      uint
)

func init() {
	flag.StringVar(&httpPort, "httpPort", "", "The port for the health endpoint")
	flag.StringVar(&redisAddress, "redisAddress", "", "The <IP:PORT> string to use for connection to a redis cache")
	flag.StringVar(&redisPassword, "redisPassword", "", "The password used for authentication to a secured redis instance")
	flag.IntVar(&db, "db", 0, "The network cache database")
	flag.UintVar(&srvPort, "srvPort", 8080, "Port to listen on")
	flag.StringVar(&bootstrapIP, "bootstrapIP", "", "Optional IP to bootstrap node against")
	flag.StringVar(&bootstrapPort, "bootstrapPort", "", "Optional port of node to bootstrap against")
	flag.StringVar(&localPort, "localPort", "8080", "Specify a different port to listen on locally")
	flag.Parse()
}

// NewServer creates a new Overlay Service Server
func NewServer(k *kademlia.Kademlia, db *redis.OverlayClient, l *zap.Logger, m *monkit.Registry) *grpc.Server {
	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{
		kad:     k,
		DB:      db,
		logger:  l,
		metrics: m,
	})

	return grpcServer
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (proto.OverlayClient, error) {
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
	// TODO
	// 1. Boostrap a node on the network
	// 2. Start up the overlay gRPC service
	// 3. Connect to Redis
	// 4. Boostrap Redis Cache

	// TODO(coyle): Should add the ability to pass a configuration to change the bootstrap node
	in := kademlia.GetIntroNode(bootstrapIP, bootstrapPort)

	kad, err := kademlia.NewKademlia([]proto.Node{in}, "bootstrap.storj.io", "8080")
	if err != nil {
		s.logger.Error("Failed to instantiate new Kademlia", zap.Error(err))
		return err
	}

	if err := kad.ListenAndServe(); err != nil {
		s.logger.Error("Failed to ListenAndServe on new Kademlia", zap.Error(err))
		return err
	}

	if err := kad.Bootstrap(ctx); err != nil {
		s.logger.Error("Failed to Bootstrap on new Kademlia", zap.Error(err))
		return err
	}

	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, kad)
	if err != nil {
		s.logger.Error("Failed to create a new redis overlay client", zap.Error(err))
		return err
	}

	if err := cache.Bootstrap(ctx); err != nil {
		s.logger.Error("Failed to boostrap cache", zap.Error(err))
		return err
	}

	// send off cache refreshes concurrently
	go cache.Refresh(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", srvPort))
	if err != nil {
		s.logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{
		kad:     kad,
		DB:      cache,
		logger:  s.logger,
		metrics: s.metrics,
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "OK") })
	go func() { http.ListenAndServe(fmt.Sprintf(":%s", httpPort), nil) }()
	go cache.Walk(ctx)

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
