// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/trust"
)

func TestPoolRequiresCachePath(t *testing.T) {
	log := zaptest.NewLogger(t)
	_, err := trust.NewPool(log, newFakeIdentityResolver(), trust.Config{})
	require.EqualError(t, err, "trust: cache path cannot be empty")
}

func TestPoolVerifySatelliteID(t *testing.T) {
	ctx, pool, source, _ := newPoolTest(t)
	defer ctx.Cleanup()

	id := testrand.NodeID()

	// Assert the ID is not trusted
	err := pool.VerifySatelliteID(context.Background(), id)
	require.EqualError(t, err, fmt.Sprintf("trust: satellite %q is untrusted", id))

	// Refresh the pool with the new trust entry
	source.entries = []trust.Entry{
		{
			SatelliteURL: trust.SatelliteURL{
				ID:   id,
				Host: "foo.test",
				Port: 7777,
			},
		},
	}
	require.NoError(t, pool.Refresh(context.Background()))

	// Assert the ID is now trusted
	err = pool.VerifySatelliteID(context.Background(), id)
	require.NoError(t, err)

	// Refresh the pool after removing the trusted satellite
	source.entries = nil
	require.NoError(t, pool.Refresh(context.Background()))

	// Assert the ID is no longer trusted
	err = pool.VerifySatelliteID(context.Background(), id)
	require.EqualError(t, err, fmt.Sprintf("trust: satellite %q is untrusted", id))
}

func TestPoolGetSignee(t *testing.T) {
	id := testrand.NodeID()
	url := trust.SatelliteURL{
		ID:   id,
		Host: "foo.test",
		Port: 7777,
	}

	ctx, pool, source, resolver := newPoolTest(t)
	defer ctx.Cleanup()

	// ID is untrusted
	_, err := pool.GetSignee(context.Background(), id)
	require.EqualError(t, err, fmt.Sprintf("trust: satellite %q is untrusted", id))

	// Refresh the pool with the new trust entry
	source.entries = []trust.Entry{{SatelliteURL: url}}
	require.NoError(t, pool.Refresh(context.Background()))

	// Identity is uncached and resolving fails
	_, err = pool.GetSignee(context.Background(), id)
	require.EqualError(t, err, "trust: no identity")

	// Now make resolving succeed
	identity := &identity.PeerIdentity{
		ID:   id,
		Leaf: &x509.Certificate{},
	}
	resolver.SetIdentity(url.NodeURL(), identity)
	signee, err := pool.GetSignee(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, signee.ID())

	// Now make resolving fail but ensure we can still get the signee since
	// the identity is cached.
	resolver.SetIdentity(url.NodeURL(), nil)
	signee, err = pool.GetSignee(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, signee.ID())

	// Now update the address on the entry and assert that the identity is
	// reset in the cache and needs to be refetched (and fails since we've
	// hampered the resolver)
	url.Host = "bar.test"
	source.entries = []trust.Entry{{SatelliteURL: url}}
	require.NoError(t, pool.Refresh(context.Background()))
	_, err = pool.GetSignee(context.Background(), id)
	require.EqualError(t, err, "trust: no identity")
}

func TestPoolGetSatellites(t *testing.T) {
	ctx, pool, source, _ := newPoolTest(t)
	defer ctx.Cleanup()

	id1 := testrand.NodeID()
	id2 := testrand.NodeID()

	// Refresh the pool with the new trust entry
	source.entries = []trust.Entry{
		{
			SatelliteURL: trust.SatelliteURL{
				ID:   id1,
				Host: "foo.test",
				Port: 7777,
			},
		},
		{
			SatelliteURL: trust.SatelliteURL{
				ID:   id2,
				Host: "bar.test",
				Port: 7777,
			},
		},
	}
	require.NoError(t, pool.Refresh(context.Background()))

	expected := []storj.NodeID{id1, id2}
	actual := pool.GetSatellites(context.Background())
	assert.ElementsMatch(t, expected, actual)
}

func TestPoolGetAddress(t *testing.T) {
	ctx, pool, source, _ := newPoolTest(t)
	defer ctx.Cleanup()

	id := testrand.NodeID()

	// Assert the ID is not trusted
	address, err := pool.GetAddress(context.Background(), id)
	require.EqualError(t, err, fmt.Sprintf("trust: satellite %q is untrusted", id))
	require.Empty(t, address)

	// Refresh the pool with the new trust entry
	source.entries = []trust.Entry{
		{
			SatelliteURL: trust.SatelliteURL{
				ID:   id,
				Host: "foo.test",
				Port: 7777,
			},
		},
	}
	require.NoError(t, pool.Refresh(context.Background()))

	// Assert the ID is now trusted and the correct address is returned
	address, err = pool.GetAddress(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "foo.test:7777", address)

	// Refresh the pool with an updated trust entry with a new address
	source.entries = []trust.Entry{
		{
			SatelliteURL: trust.SatelliteURL{
				ID:   id,
				Host: "bar.test",
				Port: 7777,
			},
		},
	}
	require.NoError(t, pool.Refresh(context.Background()))

	// Assert the ID is now trusted and the correct address is returned
	address, err = pool.GetAddress(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "bar.test:7777", address)
}

func newPoolTest(t *testing.T) (*testcontext.Context, *trust.Pool, *fakeSource, *fakeIdentityResolver) {
	ctx := testcontext.New(t)

	source := &fakeSource{}

	resolver := newFakeIdentityResolver()

	log := zaptest.NewLogger(t)
	pool, err := trust.NewPool(log, resolver, trust.Config{
		Sources:   []trust.Source{source},
		CachePath: ctx.File("trust-cache.json"),
	})
	if err != nil {
		ctx.Cleanup()
		require.NoError(t, err)
	}

	return ctx, pool, source, resolver
}

type fakeIdentityResolver struct {
	mu         sync.Mutex
	identities map[storj.NodeURL]*identity.PeerIdentity
}

func newFakeIdentityResolver() *fakeIdentityResolver {
	return &fakeIdentityResolver{
		identities: make(map[storj.NodeURL]*identity.PeerIdentity),
	}
}

func (resolver *fakeIdentityResolver) SetIdentity(url storj.NodeURL, identity *identity.PeerIdentity) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.identities[url] = identity
}

func (resolver *fakeIdentityResolver) ResolveIdentity(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()

	identity := resolver.identities[url]
	if identity == nil {
		return nil, errors.New("no identity")
	}
	return identity, nil
}
