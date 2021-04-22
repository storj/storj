// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

func TestServiceHeldAmountHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		payoutsDB := db.Payout()
		source := &fakeSource{}
		pool, err := trust.NewPool(log, newFakeIdentityResolver(), trust.Config{
			Sources:   []trust.Source{source},
			CachePath: ctx.File("trust-cache.json"),
		})
		require.NoError(t, err)

		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()
		satelliteID3 := testrand.NodeID()

		// populate pool
		source.entries = []trust.Entry{
			{
				SatelliteURL: trust.SatelliteURL{
					ID:   satelliteID1,
					Host: "foo.test",
					Port: 7777,
				},
			},
			{
				SatelliteURL: trust.SatelliteURL{
					ID:   satelliteID2,
					Host: "bar.test",
					Port: 7777,
				},
			},
			{
				SatelliteURL: trust.SatelliteURL{
					ID:   satelliteID3,
					Host: "baz.test",
					Port: 7777,
				},
			},
		}
		require.NoError(t, pool.Refresh(context.Background()))

		// add paystubs
		paystubs := []payouts.PayStub{
			{
				SatelliteID: satelliteID1,
				Period:      "2021-01",
				Held:        10,
			},
			{
				SatelliteID: satelliteID1,
				Period:      "2021-02",
				Held:        10,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-01",
				Held:        0,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-02",
				Held:        0,
			},
		}
		for _, paystub := range paystubs {
			err = payoutsDB.StorePayStub(ctx, paystub)
			require.NoError(t, err)
		}

		expected := []payouts.HeldAmountHistory{
			{
				SatelliteID: satelliteID1,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 10,
					},
					{
						Period: "2021-02",
						Amount: 10,
					},
				},
			},
			{
				SatelliteID: satelliteID2,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 0,
					},
					{
						Period: "2021-02",
						Amount: 0,
					},
				},
			},
			{
				SatelliteID: satelliteID3,
			},
		}

		service, err := payouts.NewService(log, payoutsDB, db.Reputation(), db.Satellites(), pool)
		require.NoError(t, err)

		history, err := service.HeldAmountHistory(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, history)
	})
}

type fakeSource struct {
	name    string
	static  bool
	entries []trust.Entry
	err     error
}

func (s *fakeSource) String() string {
	return s.name
}

func (s *fakeSource) Static() bool {
	return s.static
}

func (s *fakeSource) FetchEntries(context.Context) ([]trust.Entry, error) {
	return s.entries, s.err
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
