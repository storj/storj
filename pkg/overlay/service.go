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
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
	proto "storj.io/storj/protos/overlay"
)

var (
	redisAddress, redisPassword, httpPort, bootstrapIP, bootstrapPort, localPort, boltdbPath string
	db                                                                                       int
	srvPort                                                                                  uint
	tlsCertPath, tlsKeyPath, tlsHosts                                                        string
	tlsCreate, tlsOverwrite                                                                  bool
)

func init() {
	flag.StringVar(&httpPort, "httpPort", "", "The port for the health endpoint")
	flag.StringVar(&redisAddress, "redisAddress", "", "The <IP:PORT> string to use for connection to a redis cache")
	flag.StringVar(&redisPassword, "redisPassword", "", "The password used for authentication to a secured redis instance")
	flag.StringVar(&boltdbPath, "boltdbPath", "", "The path to the boltdb file that should be loaded or created")
	flag.IntVar(&db, "db", 0, "The network cache database")
	flag.UintVar(&srvPort, "srvPort", 8080, "Port to listen on")
	flag.StringVar(&bootstrapIP, "bootstrapIP", "", "Optional IP to bootstrap node against")
	flag.StringVar(&bootstrapPort, "bootstrapPort", "", "Optional port of node to bootstrap against")
	flag.StringVar(&localPort, "localPort", "8081", "Specify a different port to listen on locally")
	flag.StringVar(&tlsCertPath, "tlsCertPath", "", "TLS Certificate file")
	flag.StringVar(&tlsKeyPath, "tlsKeyPath", "", "TLS Key file")
	flag.StringVar(&tlsHosts, "tlsHosts", "", "TLS Key file")
	flag.BoolVar(&tlsCreate, "tlsCreate", false, "If true, generate a new TLS cert/key files")
	flag.BoolVar(&tlsOverwrite, "tlsOverwrite", false, "If true, overwrite existing TLS cert/key files")
}

// NewServer creates a new Overlay Service Server
func NewServer(k *kademlia.Kademlia, cache *Cache, l *zap.Logger, m *monkit.Registry) (_ *grpc.Server, _ error) {
	t := &utils.TLSFileOptions{
		CertRelPath: tlsCertPath,
		KeyRelPath:  tlsKeyPath,
		Create:      tlsCreate,
		Overwrite:   tlsOverwrite,
		Hosts:       tlsHosts,
	}

	creds, err := utils.NewServerTLSFromFile(t)
	if err != nil {
		return nil, err
	}

	credsOption := grpc.Creds(creds)
	grpcServer := grpc.NewServer(credsOption)
	proto.RegisterOverlayServer(grpcServer, &Overlay{
		kad:     k,
		cache:   cache,
		logger:  l,
		metrics: m,
	})

	return grpcServer, nil
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (proto.OverlayClient, error) {
	t := &utils.TLSFileOptions{
		CertRelPath: tlsCertPath,
		KeyRelPath:  tlsKeyPath,
		Create:      tlsCreate,
		Overwrite:   tlsOverwrite,
		Hosts:       tlsHosts,
		Client:      true,
	}

	creds, err := utils.NewClientTLSFromFile(t)
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
	// TODO
	// 1. Boostrap a node on the network
	// 2. Start up the overlay gRPC service
	// 3. Connect to Redis
	// 4. Boostrap Redis Cache

	// TODO(coyle): Should add the ability to pass a configuration to change the bootstrap node
	in := kademlia.GetIntroNode("", bootstrapIP, bootstrapPort)

	kad, err := kademlia.NewKademlia(kademlia.NewID(), []proto.Node{in}, "0.0.0.0", localPort)
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
	var cache *Cache
	if redisAddress != "" {
		cache, err = NewRedisOverlayCache(redisAddress, redisPassword, db, kad)
		if err != nil {
			s.logger.Error("Failed to create a new redis overlay client", zap.Error(err))
			return err
		}
	} else if boltdbPath != "" {
		cache, err = NewBoltOverlayCache(boltdbPath, kad)
		if err != nil {
			s.logger.Error("Failed to create a new boltdb overlay client", zap.Error(err))
			return err
		}
	} else {
		return process.ErrUsage.New("You must specify one of `--boltdbPath` or `--redisAddress`")
	}

	if boltdbPath != "" {

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

	grpcServer, err := NewServer(kad, cache, s.logger, s.metrics)
	if err != nil {
		s.logger.Error("Failed to initialize grpc server", zap.Error(err))
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "OK") })
	go func() { http.ListenAndServe(fmt.Sprintf(":%s", httpPort), mux) }()
	go cache.Walk(ctx)

	// If the passed context times out or is cancelled, shutdown the gRPC server
	go func() {
		if _, ok := <-ctx.Done(); !ok {
			grpcServer.GracefulStop()
		}
	}()

	// If `grpcServer.Serve(...)` returns an error, shutdown/cleanup the gRPC server
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
