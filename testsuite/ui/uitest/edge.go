// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/gateway-mt/auth"
	"storj.io/gateway-mt/pkg/authclient"
	"storj.io/gateway-mt/pkg/server"
	"storj.io/gateway-mt/pkg/trustedip"
	"storj.io/private/cfgstruct"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

// EdgePlanet contains defaults for testplanet with Edge.
type EdgePlanet struct {
	*testplanet.Planet

	Gateway struct {
		AccessKey string
		SecretKey string

		Addr string
	}

	Auth struct {
		Addr string
	}
}

// EdgeTest defines common args for edge testing.
type EdgeTest func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet, browser *rod.Browser)

// Edge starts a testplanet together with auth service and gateway.
func Edge(t *testing.T, test EdgeTest) {
	edgehost := os.Getenv("STORJ_TEST_EDGE_HOST")
	if edgehost == "" {
		edgehost = "127.0.0.1"
	}

	// TODO: make address not hardcoded the address selection here may
	// conflict with some automatically bound address.
	authSvcAddr := net.JoinHostPort(edgehost, strconv.Itoa(randomRange(20000, 40000)))
	authSvcAddrTLS := net.JoinHostPort(edgehost, strconv.Itoa(randomRange(20000, 40000)))

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				if dir := os.Getenv("STORJ_TEST_SATELLITE_WEB"); dir != "" {
					config.Console.StaticDir = dir
				}
				config.Console.NewOnboarding = true
				config.Console.NewBrowser = true
				// TODO: this should be dynamically set from the auth service
				config.Console.GatewayCredentialsRequestURL = "http://" + authSvcAddr
			},
		},
		NonParallel: true, // Note, do not remove this, because the code above uses same auth service addr.
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]

		gwConfig := server.Config{}
		cfgstruct.Bind(&pflag.FlagSet{}, &gwConfig, cfgstruct.UseTestDefaults())
		gwConfig.Server.Address = "127.0.0.1:0"
		gwConfig.AuthURL = "http://" + authSvcAddr
		gwConfig.InsecureLogAll = true

		authURL, err := url.Parse("http://" + authSvcAddr)
		require.NoError(t, err)
		authClient, err := authclient.New(authURL, "super-secret", 5*time.Minute)
		require.NoError(t, err)

		gateway, err := server.New(gwConfig, zaptest.NewLogger(t).Named("gateway"), nil, trustedip.NewListTrustAll(), []string{}, authClient)
		require.NoError(t, err)

		defer ctx.Check(gateway.Close)

		authConfig := auth.Config{
			Endpoint:      "http://" + gateway.Address(),
			AuthToken:     "super-secret",
			KVBackend:     "memory://",
			ListenAddr:    authSvcAddr,
			ListenAddrTLS: authSvcAddrTLS,
		}
		for _, sat := range planet.Satellites {
			authConfig.AllowedSatellites = append(authConfig.AllowedSatellites, sat.NodeURL().String())
		}

		auth, err := auth.New(ctx, zaptest.NewLogger(t).Named("auth"), authConfig, ctx.Dir("authservice"))
		require.NoError(t, err)

		defer ctx.Check(auth.Close)
		ctx.Go(func() error { return auth.Run(ctx) })
		require.NoError(t, waitForAddress(ctx, authSvcAddr, 3*time.Second))

		ctx.Go(gateway.Run)
		require.NoError(t, waitForAddress(ctx, gateway.Address(), 3*time.Second))

		// todo: use the unused endpoint below
		accessKey, secretKey, _, err := cmd.RegisterAccess(ctx, access, "http://"+authSvcAddr, false, 15*time.Second)
		require.NoError(t, err)

		edge := &EdgePlanet{}
		edge.Planet = planet
		edge.Gateway.AccessKey = accessKey
		edge.Gateway.SecretKey = secretKey
		edge.Gateway.Addr = gateway.Address()
		edge.Auth.Addr = authSvcAddr

		Browser(t, ctx, func(browser *rod.Browser) {
			test(t, ctx, edge, browser)
		})
	})
}

func randomRange(low, high int) int {
	// this generates biased crypt random numbers
	// but it uses crypt/rand to avoid potentially
	// someone seeding math/rand.
	span := high - low
	return low + int(randomUint64()&0x7FFF_FFFF)%span
}

func randomUint64() uint64 {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint64(value[:])
}

// waitForAddress will monitor starting when we are able to start the process.
func waitForAddress(ctx context.Context, address string, maxStartupWait time.Duration) error {
	defer mon.Task()(&ctx)(nil)

	start := time.Now()
	for time.Since(start) < maxStartupWait {
		if tryConnect(ctx, address) {
			return ctx.Err()
		}

		// wait a bit before retrying to reduce load
		if !sync2.Sleep(ctx, 50*time.Millisecond) {
			return ctx.Err()
		}
	}
	return fmt.Errorf("did not start in required time %v", maxStartupWait)
}

// tryConnect will try to connect to the process public address.
func tryConnect(ctx context.Context, address string) bool {
	defer mon.Task()(&ctx)(nil)

	dialer := net.Dialer{}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	// write empty byte slice to trigger refresh on connection
	_, _ = conn.Write([]byte{})
	// ignoring errors, because we only care about being able to connect
	_ = conn.Close()
	return true
}
