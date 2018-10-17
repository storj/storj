package provider_test

import (
	"context"
	"net"
	"testing"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

func TestProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var serverAddress string

	{ // Server Setup
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}

		serverAddress = listener.Addr().String()

		serverCA, err := provider.NewCA(ctx, 14, 4)
		if err != nil {
			t.Fatal(err)
		}

		serverIdentity, err := serverCA.NewIdentity()
		if err != nil {
			t.Fatal(err)
		}

		provider, err := provider.NewProvider(serverIdentity, listener, nil)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			if err := provider.Run(ctx); err != nil {
				panic(err)
			}
		}()
	}

	{ // Client connection
		clientCA, err := provider.NewCA(ctx, 14, 4)
		if err != nil {
			t.Fatal(err)
		}

		clientIdentity, err := clientCA.NewIdentity()
		if err != nil {
			t.Fatal(err)
		}

		dialer := transport.NewClient(clientIdentity)

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

		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
