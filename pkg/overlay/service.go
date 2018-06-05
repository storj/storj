// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage/redis"
)

var (
	node          string
	bootstrapIP   string
	bootstrapPort string
	stun          bool
	redisAddress  string
	redisPassword string
	db            int
	gui           bool
)

func init() {
	flag.StringVar(&node, "node", "", "Boot up a storj node")
	flag.StringVar(&bootstrapIP, "bootstrapIP", "", "Specify an IP address for a custom node to bootstrap your network with.")
	flag.StringVar(&bootstrapPort, "bootstrapPort", "", "Specify a port for a custom node to bootstrap your network with.")
	flag.BoolVar(&stun, "stun", false, "Set STUN to true")
	flag.StringVar(&redisAddress, "cache", "", "The <IP:PORT> string to use for connection to a redis cache")
	flag.StringVar(&redisPassword, "password", "", "The password used for authentication to a secured redis instance")
	flag.IntVar(&db, "db", 0, "The network cache database")
	flag.BoolVar(&gui, "gui", false, "Serve a GUI for stats and metrics on localhost:4000")
	flag.Parse()
}

// NewServer creates a new Overlay Service Server
func NewServer() *grpc.Server {
	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

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
	fmt.Println("starting up storj-node")

	var bootstrapAddress string

	if bootstrapIP != "" && bootstrapPort != "" {
		fmt.Println("bootstrapping with %s:%s", bootstrapIP, bootstrapPort)
		bootstrapAddress = fmt.Sprintf("%s:%s", bootstrapIP, bootstrapPort)
	} else {
		bootstrapAddress = fmt.Sprintf("127.0.0.1:4001")
	}

	bnode := &proto.Node{
		Address: &proto.NodeAddress{
			Address: bootstrapAddress,
		},
		// Id: "123456677234234",
	}

	nodes := []proto.Node{}
	nodes = append(nodes, *bnode)

	fmt.Println("bnode %+v\n", bnode)
	fmt.Printf("nodes %+v\n", nodes)

	kad, err := kademlia.NewKademlia(nodes, "127.0.0.1", "4000")
	fmt.Println("KAD:", kad)
	if err != nil {
		s.logger.Error("Failed to create NewKademlia", zap.Error(err))
		return err
	}

	go kad.ListenAndServe()

	time.Sleep(time.Second)

	fmt.Println("bootstrap func", kad.Bootstrap(ctx))
	kad.Bootstrap(ctx)
	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, &kad)
	if err != nil {
		s.logger.Error("Failed to create a new overlay client", zap.Error(err))
		return err
	}

	if redisAddress != "" {
		fmt.Println("starting up overlay cache")
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
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		s.logger.Error("Failed to initialize TCP connection", zap.Error(err))
		return err
	}

	grpcServer := grpc.NewServer()
	proto.RegisterOverlayServer(grpcServer, &Overlay{})

	if gui {
		fmt.Println("starting up gui on port 4000")
		http.Handle("/", http.FileServer(http.Dir("./static")))
		http.ListenAndServe(":4000", nil)
	}

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
