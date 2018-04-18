package overlay

import (
	"fmt"
	"net"

	"github.com/storj/storj/protos/overlay"
	"google.golang.org/grpc"
)

// NewServer creates a new Overlay Service Server and begins serving requests on the provided port
func NewServer(port uint32) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer()
	overlay.RegisterOverlayServer(grpcServer, &Overlay{})

	if err := grpcServer.Serve(lis); err != nil {
		return nil, err
	}

	return grpcServer, nil
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (overlay.OverlayClient, error) {
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return overlay.NewOverlayClient(conn), nil
}
