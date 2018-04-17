package overlay

import (
	"context"
	"fmt"
	"net"
	"testing"

	proto "github.com/coyle/storj/protos/overlay"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	port = 50501
)

func TestNewServer(t *testing.T) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	assert.NoError(t, err)

	srv := NewServer()
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {
	address := fmt.Sprintf(":%d", port)

	lis, err := net.Listen("tcp", address)
	assert.NoError(t, err)
	srv := NewServer()
	go srv.Serve(lis)
	defer srv.Stop()

	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
