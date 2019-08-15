// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package check

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc/drpcclient"
	"storj.io/storj/drpc/drpcserver"
	"storj.io/storj/pkg/pb"
)

type rw struct {
	io.Reader
	io.Writer
}

func TestSimple(t *testing.T) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	srv := drpcserver.New()
	srv.Register(new(impl), new(pb.DRPCServiceDescription))
	cli := pb.NewDRPCServiceClient(drpcclient.New(rw{pr1, pw2}))
	go srv.Handle(rw{pr2, pw1})

	fmt.Println("=== 1")
	out, err := cli.Method1(context.Background(), &pb.In{In: 1})
	fmt.Println("cli", out, err)

	fmt.Println("=== 2")
	stream2, err := cli.Method2(context.Background())
	defer stream2.Close()
	fmt.Println("cli", err, stream2.Send(&pb.In{In: 2}))
	out, err = stream2.CloseAndRecv()
	fmt.Println("cli", out, err)

	fmt.Println("=== 3")
	stream3, err := cli.Method3(context.Background(), &pb.In{In: 3})
	defer stream3.Close()
	fmt.Println("cli", err)
	for {
		out, err := stream3.Recv()
		fmt.Println("cli", out, err)
		if err == io.EOF {
			break
		}
	}
	stream3.Close()

	fmt.Println("=== 4")
	stream4, err := cli.Method4(context.Background())
	defer stream4.Close()
	fmt.Println("cli", stream4.Send(&pb.In{In: 4}))
	fmt.Println("cli", stream4.Send(&pb.In{In: 4}))
	fmt.Println("cli", stream4.Send(&pb.In{In: 4}))
	fmt.Println("cli", stream4.CloseSend())
	for {
		out, err := stream4.Recv()
		fmt.Println("cli", out, err)
		if err == io.EOF {
			break
		}
	}

	pr1.Close()
	pr2.Close()
	pw1.Close()
	pw2.Close()
	select {}
}

type impl struct{}

func (impl) DRPCMethod1(ctx context.Context, in *pb.In) (*pb.Out, error) {
	fmt.Println("srv", in)
	return &pb.Out{Out: 1}, nil
}

func (impl) DRPCMethod2(stream pb.DRPCService_Method2Stream) error {
	for {
		in, err := stream.Recv()
		fmt.Println("srv", in, err)
		if err == io.EOF {
			break
		}
	}
	return stream.SendAndClose(&pb.Out{Out: 2})
}

func (impl) DRPCMethod3(in *pb.In, stream pb.DRPCService_Method3Stream) error {
	fmt.Println("srv", in)
	var err error
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 3}))
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 3}))
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 3}))
	return err
}

func (impl) DRPCMethod4(stream pb.DRPCService_Method4Stream) error {
	for {
		in, err := stream.Recv()
		fmt.Println("srv", in, err)
		if err == io.EOF {
			break
		}
	}
	var err error
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 4}))
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 4}))
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 4}))
	err = errs.Combine(err, stream.Send(&pb.Out{Out: 4}))
	return err
}
