// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
)

type mockConnector struct {
	realConnector   rpc.Connector
	addressesDialed []string
	dialInstead     map[string]string
}

func (m *mockConnector) DialContext(ctx context.Context, tlsConfig *tls.Config, address string) (rpc.ConnectorConn, error) {
	m.addressesDialed = append(m.addressesDialed, address)
	replacement := m.dialInstead[address]
	if replacement == "" {
		// allow numeric ip addresses through, return errors for unexpected dns lookups
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if net.ParseIP(host) == nil {
			return nil, &net.DNSError{
				Err:        "unexpected lookup",
				Name:       address,
				Server:     "a.totally.real.dns.server.i.promise",
				IsNotFound: true,
			}
		}
		replacement = address
	}
	return m.realConnector.DialContext(ctx, tlsConfig, replacement)
}

func reformVerifierWithMockConnector(t testing.TB, sat *testplanet.Satellite, mock *mockConnector) *audit.Verifier {
	tlsOptions := sat.Dialer.TLSOptions
	newDialer := rpc.NewDefaultDialer(tlsOptions)
	mock.realConnector = newDialer.Connector
	newDialer.Connector = mock

	verifier := audit.NewVerifier(
		zaptest.NewLogger(t).Named("a-special-verifier"),
		sat.Metabase.DB,
		newDialer,
		sat.Overlay.Service,
		sat.DB.Containment(),
		sat.Orders.Service,
		sat.Identity,
		sat.Config.Audit.MinBytesPerSecond,
		sat.Config.Audit.MinDownloadTimeout,
	)
	sat.Audit.Verifier = verifier
	return verifier
}

func TestGetShareDoesNameLookupIfNecessary(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		testSatellite := planet.Satellites[0]
		audits := testSatellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(testSatellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, testSatellite, "test.bucket", "some//path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, testSatellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := testSatellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		orderLimits, privateKey, _, err := testSatellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)

		// find any non-nil limit
		var limit *pb.AddressedOrderLimit
		var orderNum int
		for i, orderLimit := range orderLimits {
			if orderLimit != nil {
				limit = orderLimit
				orderNum = i
			}
		}
		require.NotNil(t, limit)

		cachedIPAndPort := "garbageXXX#:"
		mock := &mockConnector{}
		verifier := reformVerifierWithMockConnector(t, testSatellite, mock)

		share := verifier.GetShare(ctx, limit, privateKey, cachedIPAndPort, 0, segment.Redundancy.ShareSize, orderNum)
		require.NoError(t, share.Error)
		require.Equal(t, audit.NoFailure, share.FailurePhase)

		// we expect that the cached IP and port was actually dialed
		require.Contains(t, mock.addressesDialed, cachedIPAndPort)
	})
}

func TestGetSharePrefers(t *testing.T) {
	testWithRangedLoop(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, pauseQueueing pauseQueueingFunc, runQueueingOnce runQueueingOnceFunc) {
		testSatellite := planet.Satellites[0]
		audits := testSatellite.Audit

		audits.Worker.Loop.Pause()
		pauseQueueing(testSatellite)

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, testSatellite, "test.bucket", "some//path", testData)
		require.NoError(t, err)

		err = runQueueingOnce(ctx, testSatellite)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := testSatellite.Metabase.DB.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)

		orderLimits, privateKey, _, err := testSatellite.Orders.Service.CreateAuditOrderLimits(ctx, segment, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(orderLimits), 1)

		// find any non-nil limit
		var limit *pb.AddressedOrderLimit
		var orderNum int
		for i, orderLimit := range orderLimits {
			if orderLimit != nil {
				limit = orderLimit
				orderNum = i
			}
		}
		require.NotNil(t, limit)

		// make it so that when the cached IP is dialed, we dial the "right" address,
		// but when the "right" address is dialed (meaning it came from the OrderLimit,
		// we dial something else!
		cachedIPAndPort := "ohai i am the cached ip"
		mock := &mockConnector{
			dialInstead: map[string]string{
				cachedIPAndPort:                  limit.StorageNodeAddress.Address,
				limit.StorageNodeAddress.Address: "utter.failure?!*",
			},
		}
		verifier := reformVerifierWithMockConnector(t, testSatellite, mock)

		share := verifier.GetShare(ctx, limit, privateKey, cachedIPAndPort, 0, segment.Redundancy.ShareSize, orderNum)
		require.NoError(t, share.Error)
		require.Equal(t, audit.NoFailure, share.FailurePhase)

		// we expect that the cached IP and port was actually dialed
		require.Contains(t, mock.addressesDialed, cachedIPAndPort)
		// and that the right address was never dialed directly
		require.NotContains(t, mock.addressesDialed, limit.StorageNodeAddress.Address)
	})
}
