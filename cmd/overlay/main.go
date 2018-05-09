package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/common/log"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/storage/redis"
)

var (
	redisAddress  string
	redisPassword string
	db            int
)

func main() {
	// TODO(coyle): context.WithCancel
	// TODO(coyle): metrics
	ctx := context.Background()
	// bootstrap network
	kad := kademlia.Kademlia{}

	kad.Bootstrap(ctx)
	// bootstrap cache
	cache, err := redis.NewOverlayClient(redisAddress, redisPassword, db, kad)
	if err != nil {
		// TODO(coyle): handle error
	}
	if err := cache.Bootstrap(ctx); err != nil {
		// TODO(coyle): handle error
	}

	go cache.Refresh(ctx)

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		// TODO(coyle): handle error
	}

	s := overlay.NewServer()
	go s.Serve(lis)
	defer s.GracefulStop()

	signalChan := make(chan os.Signal)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan

	// TODO(coyle): use zap logger
	log.Infof("Closing with: %v\n", sig)

}

func initalizeFlags() {
	flag.StringVar(&redisAddress, "cache", "", "The IP:PORT of the redis instance you want to connect to")
	flag.StringVar(&redisPassword, "password", "", "The optional password for accessing the redis cache")
	flag.IntVar(&db, "db", 1, "The database used by the redis network cache")

	flag.Parse()
}
