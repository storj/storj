// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscantest

import (
	"context"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/grant"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storjscan"
	"storj.io/storjscan/private/testeth"
	"storj.io/storjscan/storjscandb/storjscandbtest"
	"storj.io/uplink"
)

// Stack contains references to storjscan app and eth test network.
type Stack struct {
	Log      *zap.Logger
	App      *storjscan.App
	StartApp func() error
	CloseApp func() error
	Network  *testeth.Network
	Token    blockchain.Address
}

// Test defines common services for storjscan tests.
type Test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, stack *Stack)

// Run runs testplanet and storjscan and executes test function.
func Run(t *testing.T, test Test) {
	t.Parallel()

	databases := satellitedbtest.Databases(t)
	if len(databases) == 0 {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + dbtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + dbtest.DefaultCockroach)
	}

	config := testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		NonParallel: true,
	}

	for _, satelliteDB := range databases {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			parallel := !config.NonParallel
			if parallel {
				t.Parallel()
			}

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}
			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = t.Name()
			}

			log := testplanet.NewLogger(t)

			testmonkit.Run(context.Background(), t, func(parent context.Context) {
				defer pprof.SetGoroutineLabels(parent)
				parent = pprof.WithLabels(parent, pprof.Labels("test", t.Name()))

				timeout := config.Timeout
				if timeout == 0 {
					timeout = testcontext.DefaultTimeout
				}
				ctx := testcontext.NewWithContextAndTimeout(parent, t, timeout)
				defer ctx.Cleanup()

				// storjscan ---------
				stack := Stack{
					Log: log.Named("storjscan"),
				}

				storjscanDB, err := storjscandbtest.OpenDB(ctx, stack.Log.Named("db"), satelliteDB.MasterDB.URL, "storjscandb-"+t.Name(), "S")
				if err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(storjscanDB.Close)

				if err = storjscanDB.MigrateToLatest(ctx); err != nil {
					t.Fatalf("%+v", err)
				}

				stack.Network, err = testeth.NewNetwork()
				if err != nil {
					t.Fatalf("%+v", err)
				}
				if err = stack.Network.Start(); err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(stack.Network.Close)

				token, err := testeth.DeployToken(ctx, stack.Network, 1000000)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				stack.Token = blockchain.Address(token)

				var storjscanConfig storjscan.Config
				storjscanConfig.API.Address = "127.0.0.1:0"
				storjscanConfig.API.Keys = []string{"eu:eusecret"}
				storjscanConfig.Tokens.Endpoint = stack.Network.HTTPEndpoint()
				storjscanConfig.Tokens.Contract = stack.Token.Hex()
				storjscanConfig.TokenPrice.PriceWindow = time.Minute
				storjscanConfig.TokenPrice.Interval = time.Minute
				storjscanConfig.TokenPrice.UseTestPrices = true

				stack.App, err = storjscan.NewApp(stack.Log.Named("app"), storjscanConfig, storjscanDB)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				var run errgroup.Group
				runCtx, runCancel := context.WithCancel(ctx)

				stack.StartApp = func() error {
					storjscanConfig.API.Address = stack.App.API.Listener.Addr().String()

					stack.App, err = storjscan.NewApp(stack.Log.Named("app"), storjscanConfig, storjscanDB)
					if err != nil {
						return err
					}

					runCtx, runCancel = context.WithCancel(ctx)

					run = errgroup.Group{}
					run.Go(func() error {
						err := stack.App.Run(runCtx)
						return err
					})

					return nil
				}
				stack.CloseApp = func() error {
					runCancel()

					var errlist errs.Group
					errlist.Add(run.Wait())
					errlist.Add(stack.App.Close())
					return errlist.Err()
				}

				run.Go(func() error {
					err := stack.App.Run(runCtx)
					return err
				})
				defer ctx.Check(stack.CloseApp)
				// ------------

				planetConfig.Reconfigure = testplanet.Reconfigure{Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
					config.Payments.Storjscan.Auth.Identifier = "eu"
					config.Payments.Storjscan.Auth.Secret = "eusecret"
					config.Payments.Storjscan.Endpoint = "http://" + stack.App.API.Listener.Addr().String()
					config.Payments.Storjscan.Confirmations = 1
					config.Payments.Storjscan.DisableLoop = false
				}}

				planet, err := testplanet.NewCustom(ctx, log, planetConfig, satelliteDB)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(planet.Shutdown)

				if err = planet.Start(ctx); err != nil {
					t.Fatalf("%+v", err)
				}
				provisionUplinks(ctx, t, planet)

				test(t, ctx, planet, &stack)
			})
		})
	}
}

func provisionUplinks(ctx context.Context, t *testing.T, planet *testplanet.Planet) {
	for _, planetUplink := range planet.Uplinks {
		for _, satellite := range planet.Satellites {
			apiKey := planetUplink.APIKey[satellite.ID()]

			// create access grant manually to avoid dialing satellite for
			// project id and deriving key with argon2.IDKey method
			encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
			encAccess.SetDefaultPathCipher(storj.EncAESGCM)

			grantAccess := grant.Access{
				SatelliteAddress: satellite.URL(),
				APIKey:           apiKey,
				EncAccess:        encAccess,
			}

			serializedAccess, err := grantAccess.Serialize()
			if err != nil {
				t.Fatalf("%+v", err)
			}
			access, err := uplink.ParseAccess(serializedAccess)
			if err != nil {
				t.Fatalf("%+v", err)
			}

			planetUplink.Access[satellite.ID()] = access
		}
	}
}
