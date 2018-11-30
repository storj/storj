package kademlia_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

func newTestIdentity() (*provider.FullIdentity, error) {
	fid, err := node.NewFullIdentity(context.Background(), 12, 4)
	return fid, err
}

func TestLookupNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	k := planet.Satellites[0].Kademlia

	fmt.Printf("K: %+v\n", k)
	// err = k.Bootstrap(ctx)
	// assert.NoError(t, err)

	id, err := newTestIdentity()
	assert.NoError(t, err)
	assert.NotNil(t, id)

	nodes, err := k.GetNodes(ctx, string(id.ID), 1000, pb.Restriction{})
	assert.NoError(t, err)
	assert.NotNil(t, nodes)
	assert.NotEqual(t, len(nodes), 0)

	seen := k.Seen()
	assert.NotEqual(t, len(seen), 0)
	assert.NotNil(t, seen)
}
