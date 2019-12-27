// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/auth/grpcauth"
)

func TestAPIKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	// listener is closed in server.Stop() internally

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpcauth.InterceptAPIKey),
	)
	defer server.Stop()

	pb.RegisterGreeterServer(server, &helloServer{})

	ctx.Go(func() error {
		err := errs2.IgnoreCanceled(server.Serve(listener))
		t.Log(err)
		return err
	})

	type testcase struct {
		apikey   string
		expected codes.Code
	}

	for _, test := range []testcase{
		{"", codes.Unauthenticated},
		{"wrong key", codes.Unauthenticated},
		{"good key", codes.OK},
	} {
		conn, err := grpc.DialContext(ctx, listener.Addr().String(),
			grpc.WithPerRPCCredentials(grpcauth.NewDeprecatedAPIKeyCredentials(test.apikey)),
			grpc.WithBlock(),
			grpc.WithInsecure(),
		)
		require.NoError(t, err)

		client := pb.NewGreeterClient(conn)
		response, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Me"})

		if test.expected == codes.OK {
			require.NoError(t, err)
			require.Equal(t, "Hello Me", response.Message)
		} else {
			require.Error(t, err)
			require.Equal(t, test.expected, status.Code(err))
		}

		require.NoError(t, conn.Close())
	}
}

type helloServer struct{}

// SayHello implements helloworld.GreeterServer
func (s *helloServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	key, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credentials")
	}
	if string(key) != "good key" {
		return nil, status.Errorf(codes.Unauthenticated, "Invalid API credentials")
	}

	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}
