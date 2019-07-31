// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/auth/grpcauth"
)

func TestAPIKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	// listener is closed in server.Stop() internally

	// setup server
	serverCertificate, err := loadCertificate()
	require.NoError(t, err)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpcauth.InterceptAPIKey),
		grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates:       []tls.Certificate{serverCertificate},
			InsecureSkipVerify: true,
		})),
	)
	defer server.Stop()

	pb.RegisterGreeterServer(server, &helloServer{})
	ctx.Go(func() error {
		return errs2.IgnoreCanceled(server.Serve(listener))
	})

	time.Sleep(time.Second)

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
			grpc.WithPerRPCCredentials(grpcauth.NewAPIKeyCredentials(test.apikey)),
			grpc.WithBlock(),
			grpc.WithInsecure(),
		)
		require.NoError(t, err)

		client := pb.NewGreeterClient(conn)
		response, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Me"})

		if test.expected == codes.OK {
			require.NoError(t, err)
			require.Equal(t, response.Message, "Hello Me")
		} else {
			t.Log(err)
			require.Error(t, err)
			require.Equal(t, status.Code(err), test.expected)
		}

		fmt.Println("dialed")
		// TODO:

		require.NoError(t, conn.Close())
	}
}

type helloServer struct{}

// SayHello implements helloworld.GreeterServer
func (s *helloServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	key, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credentials")
	}
	if string(key) != "good key" {
		return nil, grpc.Errorf(codes.Unauthenticated, "Invalid API credentials")
	}

	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func loadCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(
		[]byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`),
		[]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`),
	)
}
