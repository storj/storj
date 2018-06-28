package transport

import (
	"context"

	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

// Client defines the interface to an network client.
type Client interface {
	// DialUnauthenticated connects to the provided address in an insecure manner.
	DialUnauthenticated(ctx context.Context, addr string) (node.Client, error)
	// DialNode attempts a connection to the provided node.
	DialNode(ctx context.Context, Node proto.Node) (node.Client, error)
}
