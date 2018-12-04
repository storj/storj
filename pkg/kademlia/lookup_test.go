package kademlia_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/provider"
)

func newTestIdentity() (*provider.FullIdentity, error) {
	fid, err := provider.NewFullIdentity(context.Background(), 12, 4)
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
	err = k.Bootstrap(ctx)
	assert.NoError(t, err)

	// time.Sleep(5 * time.Second)

	id, err := newTestIdentity()
	assert.NoError(t, err)
	assert.NotNil(t, id)

	// _, err = k.GetNodes(ctx, id.ID, 1000, pb.Restriction{})
	// assert.NoError(t, err)

	seen := k.Seen()
	assert.NotEqual(t, len(seen), 0)
	assert.NotNil(t, seen)

	target := seen[0]
	found, err := k.FindNode(ctx, target.Id)
	assert.NoError(t, err)
	assert.Equal(t, target.Id, found.Id)
}
