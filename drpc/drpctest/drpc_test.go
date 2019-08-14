// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpctest

import (
	"context"
	"fmt"
	"io"
	"testing"

	"storj.io/storj/drpc/drpcclient"
	"storj.io/storj/drpc/drpcserver"
	"storj.io/storj/pkg/pb"
)

type rwc struct {
	io.Reader
	io.Writer
	io.Closer
}

func TestSimple(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	srv := drpcserver.New()
	srv.Register(new(impl), new(pb.DRPCServiceDescription))
	cli := pb.NewDRPCServiceClient(drpcclient.New(rwc{pr1, pw2, pr1}))
	go srv.Handle(rwc{pr2, pw1, pr2})

	fmt.Println("=== 1")
	out, err := cli.Method1(context.Background(), &pb.In{In: 1})
	fmt.Println("cli", out, err)

	fmt.Println("=== 2")
	stream2, err := cli.Method2(context.Background())
	fmt.Println("cli", err, stream2.Send(&pb.In{In: 2}))
	out, err = stream2.CloseAndRecv()
	fmt.Println("cli", out, err)

	fmt.Println("=== 3")
	stream3, err := cli.Method3(context.Background(), &pb.In{In: 3})
	fmt.Println("cli", err)
	out, err = stream3.Recv()
	fmt.Println("cli", out, err)
	out, err = stream3.Recv()
	fmt.Println("cli", out, err)
}

type impl struct{}

func (impl) DRPCMethod1(ctx context.Context, in *pb.In) (*pb.Out, error) {
	fmt.Println("srv", in)
	return &pb.Out{Out: 1}, nil
}

func (impl) DRPCMethod2(stream pb.DRPCService_Method2Stream) error {
	in, err := stream.Recv()
	fmt.Println("srv", in, err)
	return stream.SendAndClose(&pb.Out{Out: 2})
}

func (impl) DRPCMethod3(in *pb.In, stream pb.DRPCService_Method3Stream) error {
	fmt.Println("srv", in)
	stream.Send(&pb.Out{Out: 3})
	stream.Send(&pb.Out{Out: 3})
	return nil
}

func (impl) DRPCMethod4(stream pb.DRPCService_Method4Stream) error {
	in, err := stream.Recv()
	fmt.Println("srv", in, err)
	return stream.Send(&pb.Out{Out: 2})
}
