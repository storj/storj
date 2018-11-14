package provider_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

//go:generate protoc --go_out=plugins=grpc:. ping_test.proto
//go:generate rm ping_test.pb_test.go
//go:generate mv ping_test.pb.go ping_test.pb_test.go

func TestPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var serverAddress string

	{ // Server Setup
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}

		serverAddress = listener.Addr().String()

		ca, err := provider.NewTestCA(ctx)
		if err != nil {
			t.Fatal(err)
		}

		identity, err := ca.NewIdentity()
		if err != nil {
			t.Fatal(err)
		}

		provider, err := provider.NewProvider(identity, listener, nil)
		if err != nil {
			t.Fatal(err)
		}

		RegisterPingServiceServer(provider.GRPC(), &Server{})

		go func() {
			if err := provider.Run(ctx); err != nil {
				panic(err)
			}
		}()
	}

	{ // Client connection
		ca, err := provider.NewTestCA(ctx)
		if err != nil {
			t.Fatal(err)
		}

		identity, err := ca.NewIdentity()
		if err != nil {
			t.Fatal(err)
		}

		dialer := transport.NewClient(identity)

		conn, err := dialer.DialNode(ctx, &pb.Node{
			Id: "",
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   serverAddress,
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
		}()

		client := NewPingServiceClient(conn)
		if err != nil {
			t.Fatal(err)
		}

		rep, err := client.Ping(ctx, &Request{Text: "hello"})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("ping response %v", rep)
	}
}

type Server struct{}

func (p *Server) Ping(ctx context.Context, req *Request) (*Response, error) {
	return &Response{Text: fmt.Sprintf("%s - pong", req.Text)}, nil
}
