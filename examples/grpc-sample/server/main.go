// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
)

var (
	port int
)

type sampleServer struct {
}

func initializeFlags() {
	flag.IntVar(&port, "port", 8080, "port")
	flag.Parse()
}

func (s *sampleServer) SampleBidirectional(stream pb.Sample_SampleBidirectionalServer) error {
	for {
		res, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		fmt.Println("received", res.GetData())
		err = stream.Send(&pb.SampleBidirectionalResponse{
			Data: res.GetData(),
		})
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	return nil
}

func main() {
	initializeFlags()

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("failed")
		os.Exit(1)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()

	var s sampleServer
	pb.RegisterSampleServer(grpcServer, &s)

	defer grpcServer.GracefulStop()
	err = grpcServer.Serve(lis)
	if err != nil {
		fmt.Println("failed")
		os.Exit(1)
	}
}
