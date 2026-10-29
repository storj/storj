// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscantest

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/grant"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqtest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
	"storj.io/storjscan"
	"storj.io/storjscan/common"
	"storj.io/storjscan/private/testeth"
	"storj.io/storjscan/storjscandb"
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

			jobqtest.WithServer(t, &jobqtest.ServerOptions{
				Timeout: config.Timeout,
			}, func(ctx *testcontext.Context, jobqServer *jobqtest.TestServer) {
				// storjscan ---------
				stack := Stack{
					Log: log.Named("storjscan"),
				}

				storjscanDB, err := openStorjscanDB(ctx, stack.Log.Named("db"), satelliteDB.MasterDB.URL, "storjscandb-"+t.Name(), "S")
				if err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(storjscanDB.Close)

				if err = storjscanDB.MigrateToLatest(ctx); err != nil {
					t.Fatalf("%+v", err)
				}

				stack.Network, err = newNetwork()
				if err != nil {
					t.Fatalf("%+v", err)
				}
				if err = stack.Network.Start(); err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(stack.Network.Close)

				if err = testeth.DeployToken(ctx, stack.Network, 1000000); err != nil {
					t.Fatalf("%+v", err)
				}
				stack.Token = blockchain.Address(stack.Network.TokenAddress())

				endpoints, err := json.Marshal([]common.EthEndpoint{{
					Name:     "Geth",
					URL:      stack.Network.HTTPEndpoint(),
					Contract: stack.Token.Hex(),
					ChainID:  stack.Network.ChainID().Int64(),
					Currency: "STORJ",
				}})
				if err != nil {
					t.Fatalf("%+v", err)
				}

				var storjscanConfig storjscan.Config
				storjscanConfig.API.Address = "127.0.0.1:0"
				storjscanConfig.API.Keys = []string{"eu:eusecret"}
				storjscanConfig.Tokens.Endpoints = string(endpoints)
				storjscanConfig.Events.AddressBatchSize = 100
				storjscanConfig.Events.BlockBatchSize = 5000
				storjscanConfig.Events.ChainReorgBuffer = 15
				storjscanConfig.Events.MaximumQuerySize = 10000
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
						err := runApp(runCtx, stack.App)
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
					err := runApp(runCtx, stack.App)
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
					config.JobQueue = jobq.Config{
						ServerNodeURL: jobqServer.NodeURL,
						TLS:           jobqServer.TLSOpts.Config,
					}
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

// runApp runs storjscan like App.Run, but without verifying the token decimals:
// the test token has 18 decimals, which doesn't match the STORJ currency.
func runApp(ctx context.Context, app *storjscan.App) error {
	group, ctx := errgroup.WithContext(ctx)
	app.Servers.Run(ctx, group)
	app.Services.Run(ctx, group)
	return group.Wait()
}

// newNetwork creates an in-process Ethereum test network, configured the same
// way as testeth.Run.
func newNetwork() (*testeth.Network, error) {
	nodeConfig := node.DefaultConfig
	nodeConfig.Name = "testeth"
	nodeConfig.DataDir = ""
	nodeConfig.HTTPHost = "127.0.0.1"
	nodeConfig.HTTPPort = 0
	nodeConfig.AuthPort = 0
	nodeConfig.HTTPModules = append(nodeConfig.HTTPModules, "eth")
	nodeConfig.P2P.MaxPeers = 0
	nodeConfig.P2P.ListenAddr = ""
	nodeConfig.P2P.NoDial = true
	nodeConfig.P2P.NoDiscovery = true
	nodeConfig.P2P.DiscoveryV5 = false

	ethConfig := ethconfig.Defaults
	ethConfig.NetworkId = 1337
	ethConfig.SyncMode = ethconfig.FullSync
	ethConfig.Miner.GasPrice = big.NewInt(params.GWei)
	ethConfig.FilterLogCacheSize = 100

	return testeth.NewNetwork(nodeConfig, ethConfig, 2)
}

// storjscanTestDB is a storjscan database with a unique schema which is
// dropped on close.
type storjscanTestDB struct {
	*storjscandb.DB
	tempDB *dbutil.TempDatabase
}

// openStorjscanDB opens a new storjscan database in a unique temporary schema.
// It replaces storjscandbtest.OpenDB, which is built against a different
// storj.io/storj tempdb API.
func openStorjscanDB(ctx context.Context, log *zap.Logger, connStr, testName, category string) (*storjscanTestDB, error) {
	schemaSuffix := pgutil.CreateRandomTestingSchemaName(6)
	maxTestNameLength := 64 - len(schemaSuffix) - len(category)
	if len(testName) > maxTestNameLength {
		testName = testName[:maxTestNameLength]
	}
	schemaName := strings.ToLower(testName + "/" + schemaSuffix + "/" + category)

	tempDB, err := tempdb.OpenUnique(ctx, log.Named("tempdb"), connStr, schemaName)
	if err != nil {
		return nil, err
	}
	storjscanDB, err := storjscandb.Open(ctx, log, tempDB.ConnStr)
	if err != nil {
		return nil, errs.Combine(err, tempDB.Close())
	}
	return &storjscanTestDB{DB: storjscanDB, tempDB: tempDB}, nil
}

// Close closes the database and drops the temporary schema.
func (db *storjscanTestDB) Close() error {
	return errs.Combine(db.DB.Close(), db.tempDB.Close())
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
