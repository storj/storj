// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

func TestServiceHeldAmountHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		log := zaptest.NewLogger(t)
		payoutsDB := db.Payout()
		satellitesDB := db.Satellites()
		source := &fakeSource{}
		pool, err := trust.NewPool(log, newFakeIdentityResolver(), trust.Config{
			Sources:   []trust.Source{source},
			CachePath: ctx.File("trust-cache.json"),
		}, satellitesDB)
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
		require.NoError(t, pool.Refresh(t.Context()))

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

func TestService_AllSatellitesPayoutPeriod(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1, SatelliteCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		untrustedSatelliteID := testrand.NodeID()

		payoutsDB := planet.StorageNodes[0].DB.Payout()
		period := "2023-12"

		paystub1 := payouts.PayStub{
			SatelliteID:    satellite1.ID(),
			Period:         period,
			Created:        time.Now().UTC(),
			Codes:          "qwe",
			UsageAtRest:    1,
			UsageGet:       2,
			UsagePut:       3,
			UsageGetRepair: 4,
			UsagePutRepair: 5,
			UsageGetAudit:  6,
			CompAtRest:     7,
			CompGet:        8,
			CompPut:        9,
			CompGetRepair:  10,
			CompPutRepair:  11,
			CompGetAudit:   12,
			SurgePercent:   13,
			Held:           14,
			Owed:           15,
			Disposed:       16,
			Paid:           17,
		}

		paystub2 := payouts.PayStub{
			SatelliteID:    untrustedSatelliteID,
			Period:         period,
			Created:        time.Now().UTC(),
			Codes:          "qwe",
			UsageAtRest:    1,
			UsageGet:       2,
			UsagePut:       3,
			UsageGetRepair: 4,
			UsagePutRepair: 5,
			UsageGetAudit:  6,
			CompAtRest:     7,
			CompGet:        8,
			CompPut:        9,
			CompGetRepair:  10,
			CompPutRepair:  11,
			CompGetAudit:   12,
			SurgePercent:   13,
			Held:           14,
			Owed:           15,
			Disposed:       16,
			Paid:           17,
		}

		require.NoError(t, payoutsDB.StorePayStub(ctx, paystub1))
		require.NoError(t, payoutsDB.StorePayStub(ctx, paystub2))

		rep, err := planet.StorageNodes[0].DB.Reputation().Get(ctx, satellite1.ID())
		require.NoError(t, err)
		satellite1Payout, err := payouts.PaystubToSatellitePayoutForPeriod(paystub1, rep.JoinedAt, "", satellite1.NodeURL().Address, false)
		require.NoError(t, err)

		satellite2Payout, err := payouts.PaystubToSatellitePayoutForPeriod(paystub2, time.Time{}, "", untrustedSatelliteID.String(), false)
		require.NoError(t, err)

		payoutService := planet.StorageNodes[0].Payout.Service

		expectedSatellitePayouts, err := payoutService.AllSatellitesPayoutPeriod(ctx, period)
		require.NoError(t, err)
		require.Equal(t, 2, len(expectedSatellitePayouts))

		for _, payout := range expectedSatellitePayouts {
			switch payout.SatelliteID {
			case satellite1.ID().String():
				require.Equal(t, satellite1Payout, payout)
			case untrustedSatelliteID.String():
				require.Equal(t, satellite2Payout, payout)
			default:
				require.Fail(t, "unexpected satellite")
			}
		}
	})
}
