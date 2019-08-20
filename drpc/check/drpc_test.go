// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package check

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"storj.io/storj/drpc/drpcconn"
	"storj.io/storj/drpc/drpcserver"
	"storj.io/storj/pkg/pb"
)

func TestSimple(t *testing.T) {
	type rw struct {
		io.Reader
		io.Writer
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	defer pr1.Close()
	defer pr2.Close()

	srv := drpcserver.New()
	srv.Register(new(impl), new(pb.DRPCServiceDescription))
	go srv.Manage(ctx, rw{pr2, pw1})

	conn := drpcconn.New(ctx, rw{pr1, pw2})
	cli := pb.NewDRPCServiceClient(conn)
	defer conn.Close()

	{
		fmt.Println("=== 1")
		out, err := cli.Method1(ctx, &pb.In{In: 1})
		fmt.Println("CLI 0 =>", err, out)
	}

	time.Sleep(time.Second)

	{
		fmt.Println("=== 2")
		stream2, err := cli.Method2(ctx)
		fmt.Println("CLI 0 =>", err)
		fmt.Println("CLI 1 =>", stream2.Send(&pb.In{In: 2}))
		fmt.Println("CLI 2 =>", stream2.Send(&pb.In{In: 2}))
		out, err := stream2.CloseAndRecv()
		fmt.Println("CLI 3 =>", err, out)
		fmt.Println("CLI 4 =>", stream2.Close())
	}

	time.Sleep(time.Second)

	{
		fmt.Println("=== 3")
		stream3, err := cli.Method3(ctx, &pb.In{In: 3})
		fmt.Println("CLI 0 =>", err)
		for {
			out, err := stream3.Recv()
			fmt.Println("CLI 1 =>", err, out)
			if err != nil {
				break
			}
		}
		fmt.Println("CLI 2 =>", stream3.Close())
	}

	time.Sleep(time.Second)

	{
		fmt.Println("=== 4")
		stream4, err := cli.Method4(ctx)
		fmt.Println("CLI 0 =>", err)
		fmt.Println("CLI 1 =>", stream4.Send(&pb.In{In: 4}))
		fmt.Println("CLI 2 =>", stream4.Send(&pb.In{In: 4}))
		fmt.Println("CLI 3 =>", stream4.Send(&pb.In{In: 4}))
		fmt.Println("CLI 4 =>", stream4.Send(&pb.In{In: 5}))
		fmt.Println("CLI 5 =>", stream4.CloseSend())
		for {
			out, err := stream4.Recv()
			fmt.Println("CLI 6 =>", err, out)
			if err != nil {
				break
			}
		}
		fmt.Println("CLI 7 =>", stream4.Close())
	}
}

type impl struct{}

func (impl) DRPCMethod1(ctx context.Context, in *pb.In) (*pb.Out, error) {
	fmt.Println("SRV 0 <=", in)
	return &pb.Out{Out: 1}, nil
}

func (impl) DRPCMethod2(stream pb.DRPCService_Method2Stream) error {
	for {
		in, err := stream.Recv()
		fmt.Println("SRV 0 <=", err, in)
		if err != nil {
			break
		}
	}
	err := stream.SendAndClose(&pb.Out{Out: 2})
	fmt.Println("SRV 1 <=", err)
	return err
}

func (impl) DRPCMethod3(in *pb.In, stream pb.DRPCService_Method3Stream) error {
	fmt.Println("SRV 0 <=", in)
	fmt.Println("SRV 1 <=", stream.Send(&pb.Out{Out: 3}))
	fmt.Println("SRV 2 <=", stream.Send(&pb.Out{Out: 3}))
	fmt.Println("SRV 3 <=", stream.Send(&pb.Out{Out: 3}))
	return nil
}

func (impl) DRPCMethod4(stream pb.DRPCService_Method4Stream) error {
	for {
		in, err := stream.Recv()
		fmt.Println("SRV 0 <=", err, in)
		if err != nil {
			break
		}
	}
	fmt.Println("SRV 1 <=", stream.Send(&pb.Out{Out: 4}))
	fmt.Println("SRV 2 <=", stream.Send(&pb.Out{Out: 4}))
	fmt.Println("SRV 3 <=", stream.Send(&pb.Out{Out: 4}))
	fmt.Println("SRV 4 <=", stream.Send(&pb.Out{Out: 4}))
	return nil
}
