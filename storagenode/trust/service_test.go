// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"crypto/x509"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

func TestPoolRequiresCachePath(t *testing.T) {
	log := zaptest.NewLogger(t)
	_, err := trust.NewPool(log, newFakeIdentityResolver(), trust.Config{}, nil)
	require.EqualError(t, err, "trust: cache path cannot be empty")
}

func TestPoolVerifySatelliteID(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		pool, source, _ := newPoolTest(ctx, t, db)

		id := testrand.NodeID()

		// Assert the ID is not trusted
		err := pool.VerifySatelliteID(t.Context(), id)
		require.ErrorIs(t, err, trust.ErrUntrusted)

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
		require.NoError(t, pool.Refresh(t.Context()))

		// Assert the ID is now trusted
		err = pool.VerifySatelliteID(t.Context(), id)
		require.NoError(t, err)

		// Refresh the pool after removing the trusted satellite
		source.entries = nil
		require.NoError(t, pool.Refresh(t.Context()))

		// Assert the ID is no longer trusted
		err = pool.VerifySatelliteID(t.Context(), id)
		require.ErrorIs(t, err, trust.ErrUntrusted)
	})
}

func TestPoolGetSignee(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		id := testrand.NodeID()
		url := trust.SatelliteURL{
			ID:   id,
			Host: "foo.test",
			Port: 7777,
		}

		pool, source, resolver := newPoolTest(ctx, t, db)

		// ID is untrusted
		_, err := pool.GetSignee(t.Context(), id)
		require.ErrorIs(t, err, trust.ErrUntrusted)

		// Refresh the pool with the new trust entry
		source.entries = []trust.Entry{{SatelliteURL: url}}
		require.NoError(t, pool.Refresh(t.Context()))

		// Identity is uncached and resolving fails
		_, err = pool.GetSignee(t.Context(), id)
		require.EqualError(t, err, "trust: no identity")

		// Now make resolving succeed
		identity := &identity.PeerIdentity{
			ID:   id,
			Leaf: &x509.Certificate{},
		}
		resolver.SetIdentity(url.NodeURL(), identity)
		signee, err := pool.GetSignee(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, id, signee.ID())

		// Now make resolving fail but ensure we can still get the signee since
		// the identity is cached.
		resolver.SetIdentity(url.NodeURL(), nil)
		signee, err = pool.GetSignee(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, id, signee.ID())

		// Now update the address on the entry and assert that the identity is
		// reset in the cache and needs to be refetched (and fails since we've
		// hampered the resolver)
		url.Host = "bar.test"
		source.entries = []trust.Entry{{SatelliteURL: url}}
		require.NoError(t, pool.Refresh(t.Context()))
		_, err = pool.GetSignee(t.Context(), id)
		require.EqualError(t, err, "trust: no identity")
	})
}

func TestPoolGetSatellites(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		pool, source, _ := newPoolTest(ctx, t, db)

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
		require.NoError(t, pool.Refresh(t.Context()))

		expected := []storj.NodeID{id1, id2}
		actual := pool.GetSatellites(t.Context())
		assert.ElementsMatch(t, expected, actual)
	})
}

func TestPool_SatelliteDB_Status(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		source := &fakeSource{}

		resolver := newFakeIdentityResolver()

		log := zaptest.NewLogger(t)
		config := trust.Config{
			Sources:         []trust.Source{source},
			CachePath:       ctx.File("trust-cache.json"),
			RefreshInterval: 0 * time.Second,
		}

		pool, err := trust.NewPool(log, resolver, config, db.Satellites())
		require.NoError(t, err)

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

		require.NoError(t, pool.Refresh(t.Context()))

		sats, err := db.Satellites().GetSatellites(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, len(sats))
		require.Equal(t, satellites.Normal, sats[0].Status)
		require.Equal(t, satellites.Normal, sats[1].Status)

		// Refresh the pool with the new trust entry
		source.entries = []trust.Entry{
			{
				SatelliteURL: trust.SatelliteURL{
					ID:   id2,
					Host: "bar.test",
					Port: 7777,
				},
			},
		}
		require.NoError(t, pool.Refresh(t.Context()))
		sats, err = db.Satellites().GetSatellites(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, len(sats))

		for i := 0; i < len(sats); i++ {
			switch sats[i].SatelliteID {
			case id1:
				require.Equal(t, satellites.Untrusted, sats[i].Status)
			case id2:
				require.Equal(t, satellites.Normal, sats[i].Status)
			default:
				t.Fatal("unexpected satellite")
			}
		}

		expected := []storj.NodeID{id2}
		actual := pool.GetSatellites(t.Context())
		assert.ElementsMatch(t, expected, actual)

		// test cases when the untrusted satellite is now trusted
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

		require.NoError(t, pool.Refresh(t.Context()))
		sats, err = db.Satellites().GetSatellites(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, len(sats))
		require.Equal(t, satellites.Normal, sats[0].Status)
		require.Equal(t, satellites.Normal, sats[1].Status)

		expected = []storj.NodeID{id1, id2}
		actual = pool.GetSatellites(t.Context())
		assert.ElementsMatch(t, expected, actual)
	})
}

func TestPoolGetAddress(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		pool, source, _ := newPoolTest(ctx, t, db)

		id := testrand.NodeID()

		// Assert the ID is not trusted
		nodeurl, err := pool.GetNodeURL(t.Context(), id)
		require.ErrorIs(t, err, trust.ErrUntrusted)
		require.Empty(t, nodeurl)

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
		require.NoError(t, pool.Refresh(t.Context()))

		// Assert the ID is now trusted and the correct address is returned
		nodeurl, err = pool.GetNodeURL(t.Context(), id)
		require.NoError(t, err)
		require.Equal(t, id, nodeurl.ID)
		require.Equal(t, "foo.test:7777", nodeurl.Address)

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
		require.NoError(t, pool.Refresh(t.Context()))

		// Assert the ID is now trusted and the correct address is returned
		nodeurl, err = pool.GetNodeURL(t.Context(), id)
		require.NoError(t, err)
		require.Equal(t, id, nodeurl.ID)
		require.Equal(t, "bar.test:7777", nodeurl.Address)
	})
}

func newPoolTest(ctx *testcontext.Context, t *testing.T, db storagenode.DB) (*trust.Pool, *fakeSource, *fakeIdentityResolver) {
	source := &fakeSource{}

	resolver := newFakeIdentityResolver()

	log := zaptest.NewLogger(t)
	pool, err := trust.NewPool(log, resolver, trust.Config{
		Sources:   []trust.Source{source},
		CachePath: ctx.File("trust-cache.json"),
	}, db.Satellites())
	if err != nil {
		ctx.Cleanup()
		require.NoError(t, err)
	}

	return pool, source, resolver
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
