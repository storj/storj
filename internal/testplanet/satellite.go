// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/dbcleanup"
	"storj.io/storj/satellite/discovery"
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/marketingweb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/satellitedb"
)

// newSatellites initializes satellites
func (planet *Planet) newSatellites(count int) ([]*SatelliteSystem, error) {
	var xs []*SatelliteSystem
	defer func() {
		for _, x := range xs {
			planet.peers = append(planet.peers, closablePeer{peer: x})
		}
	}()

	for i := 0; i < count; i++ {
		prefix := "satellite" + strconv.Itoa(i)
		log := planet.log.Named(prefix)

		storageDir := filepath.Join(planet.directory, prefix)
		if err := os.MkdirAll(storageDir, 0700); err != nil {
			return nil, err
		}

		identity, err := planet.NewIdentity()
		if err != nil {
			return nil, err
		}

		var db satellite.DB
		if planet.config.Reconfigure.NewSatelliteDB != nil {
			db, err = planet.config.Reconfigure.NewSatelliteDB(log.Named("db"), i)
		} else {
			db, err = satellitedb.NewInMemory(log.Named("db"))
		}
		if err != nil {
			return nil, err
		}
		// TODO: add support for sqlite since boltdb cannot handle more than one
		// connection at a time
		metaInfoDBURL := "boltdb://" + filepath.Join(storageDir, "pointers.db")
		revocationDBURL := "boltdb://" + filepath.Join(storageDir, "revocation.db")
		config := satellite.Config{
			Server: server.Config{
				Address:        "127.0.0.1:0",
				PrivateAddress: "127.0.0.1:0",

				Config: tlsopts.Config{
					RevocationDBURL:     revocationDBURL,
					UsePeerCAWhitelist:  true,
					PeerCAWhitelistPath: planet.whitelistPath,
					PeerIDVersions:      "latest",
					Extensions: extensions.Config{
						Revocation:          false,
						WhitelistSignedLeaf: false,
					},
				},
			},
			Overlay: overlay.Config{
				Node: overlay.NodeSelectionConfig{
					UptimeCount:       0,
					AuditCount:        0,
					NewNodePercentage: 0,
					OnlineWindow:      0,
					DistinctIP:        false,

					AuditReputationRepairWeight:  1,
					AuditReputationUplinkWeight:  1,
					AuditReputationAlpha0:        1,
					AuditReputationBeta0:         0,
					AuditReputationLambda:        0.95,
					AuditReputationWeight:        1,
					AuditReputationDQ:            0.6,
					UptimeReputationRepairWeight: 1,
					UptimeReputationUplinkWeight: 1,
					UptimeReputationAlpha0:       2,
					UptimeReputationBeta0:        0,
					UptimeReputationLambda:       0.99,
					UptimeReputationWeight:       1,
					UptimeReputationDQ:           0.6,
				},
				UpdateStatsBatchSize: 100,
			},
			Discovery: discovery.Config{
				RefreshInterval:    1 * time.Second,
				RefreshLimit:       100,
				RefreshConcurrency: 2,
			},
			Metainfo: metainfo.Config{
				DatabaseURL:          metaInfoDBURL,
				MinRemoteSegmentSize: 0, // TODO: fix tests to work with 1024
				MaxInlineSegmentSize: 8000,
				MaxCommitInterval:    1 * time.Hour,
				Overlay:              true,
				RS: metainfo.RSConfig{
					MaxSegmentSize:   64 * memory.MiB,
					MaxBufferMem:     memory.Size(256),
					ErasureShareSize: memory.Size(256),
					MinThreshold:     (planet.config.StorageNodeCount * 1 / 5),
					RepairThreshold:  (planet.config.StorageNodeCount * 2 / 5),
					SuccessThreshold: (planet.config.StorageNodeCount * 3 / 5),
					MaxThreshold:     (planet.config.StorageNodeCount * 4 / 5),
					Validate:         false,
				},
				Loop: metainfo.LoopConfig{
					CoalesceDuration: 1 * time.Second,
				},
			},
			Orders: orders.Config{
				Expiration: 7 * 24 * time.Hour,
			},
			Checker: checker.Config{
				Interval:                  defaultInterval,
				IrreparableInterval:       defaultInterval,
				ReliabilityCacheStaleness: 1 * time.Minute,
			},
			Repairer: repairer.Config{
				MaxRepair:                     10,
				Interval:                      time.Hour,
				Timeout:                       1 * time.Minute, // Repairs can take up to 10 seconds. Leaving room for outliers
				MaxBufferMem:                  4 * memory.MiB,
				MaxExcessRateOptimalThreshold: 0.05,
			},
			Audit: audit.Config{
				MaxRetriesStatDB:   0,
				MinBytesPerSecond:  1 * memory.KB,
				MinDownloadTimeout: 5 * time.Second,
				MaxReverifyCount:   3,
				ChoreInterval:      defaultInterval,
				QueueInterval:      defaultInterval,
				Slots:              3,
				WorkerConcurrency:  1,
			},
			GarbageCollection: gc.Config{
				Interval:          defaultInterval,
				Enabled:           true,
				InitialPieces:     10,
				FalsePositiveRate: 0.1,
				ConcurrentSends:   1,
			},
			DBCleanup: dbcleanup.Config{
				SerialsInterval: defaultInterval,
			},
			Tally: tally.Config{
				Interval: defaultInterval,
			},
			Rollup: rollup.Config{
				Interval:      defaultInterval,
				MaxAlphaUsage: 25 * memory.GB,
				DeleteTallies: false,
			},
			Mail: mailservice.Config{
				SMTPServerAddress: "smtp.mail.test:587",
				From:              "Labs <storj@mail.test>",
				AuthType:          "simulate",
				TemplatePath:      filepath.Join(developmentRoot, "web/satellite/static/emails"),
			},
			Console: consoleweb.Config{
				Address:         "127.0.0.1:0",
				StaticDir:       filepath.Join(developmentRoot, "web/satellite"),
				PasswordCost:    console.TestPasswordCost,
				AuthTokenSecret: "my-suppa-secret-key",
			},
			Marketing: marketingweb.Config{
				Address:   "127.0.0.1:0",
				StaticDir: filepath.Join(developmentRoot, "web/marketing"),
			},
			Version: planet.NewVersionConfig(),
		}
		if planet.config.Reconfigure.Satellite != nil {
			planet.config.Reconfigure.Satellite(log, i, &config)
		}

		versionInfo := planet.NewVersionInfo()

		revocationDB, err := revocation.NewDBFromCfg(config.Server.Config)
		if err != nil {
			return xs, errs.Wrap(err)
		}
		planet.databases = append(planet.databases, revocationDB)

		peer, err := satellite.New(log, identity, db, revocationDB, &config, versionInfo)
		if err != nil {
			return xs, err
		}

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}
		planet.databases = append(planet.databases, db)

		api, err := planet.newAPI(i, identity, config, versionInfo)
		if err != nil {
			return xs, err
		}
		log.Debug("id=" + peer.ID().String() + " addr=" + api.Addr())

		system := createNewSystem(log, peer, api)
		xs = append(xs, system)
	}
	return xs, nil
}

// createNewSystem makes a new Satellite System and exposes the same interface from
// before we split out the API. In the short term this will help keep all the tests passing
// without much modification needed. However long term, we probably want to rework this
// so it represents how the satellite will run when it is made up of many prrocesses.
func createNewSystem(log *zap.Logger, peer *satellite.Peer, api *satellite.API) *SatelliteSystem {
	system := &SatelliteSystem{
		Peer: peer,
		API:  api,
	}
	system.Log = log
	system.Identity = peer.Identity
	system.DB = peer.DB

	system.Dialer = api.Dialer
	system.Server = api.Server
	system.Version = peer.Version

	system.Contact.Service = api.Contact.Service
	system.Contact.Endpoint = api.Contact.Endpoint
	system.Contact.KEndpoint = api.Contact.KEndpoint

	system.Overlay.DB = api.Overlay.DB
	system.Overlay.Service = api.Overlay.Service
	system.Overlay.Inspector = api.Overlay.Inspector

	system.Discovery.Service = peer.Discovery.Service

	system.Metainfo.Database = api.Metainfo.Database
	system.Metainfo.Service = api.Metainfo.Service
	system.Metainfo.Endpoint2 = api.Metainfo.Endpoint2
	system.Metainfo.Loop = peer.Metainfo.Loop

	system.Inspector.Endpoint = api.Inspector.Endpoint

	system.Orders.Endpoint = api.Orders.Endpoint
	system.Orders.Service = api.Orders.Service

	system.Repair.Checker = peer.Repair.Checker
	system.Repair.Repairer = peer.Repair.Repairer
	system.Repair.Inspector = api.Repair.Inspector

	system.Audit.Queue = peer.Audit.Queue
	system.Audit.Worker = peer.Audit.Worker
	system.Audit.Chore = peer.Audit.Chore
	system.Audit.Verifier = peer.Audit.Verifier
	system.Audit.Reporter = peer.Audit.Reporter

	system.GarbageCollection.Service = peer.GarbageCollection.Service

	system.DBCleanup.Chore = peer.DBCleanup.Chore

	system.Accounting.Tally = peer.Accounting.Tally
	system.Accounting.Rollup = peer.Accounting.Rollup
	system.Accounting.ProjectUsage = peer.Accounting.ProjectUsage

	system.LiveAccounting.Service = api.LiveAccounting.Service

	system.Mail.Service = api.Mail.Service

	system.Vouchers.Endpoint = api.Vouchers.Endpoint

	system.Console.Listener = api.Console.Listener
	system.Console.Service = api.Console.Service
	system.Console.Endpoint = api.Console.Endpoint

	system.Marketing.Listener = api.Marketing.Listener
	system.Marketing.Endpoint = api.Marketing.Endpoint

	system.NodeStats.Endpoint = api.NodeStats.Endpoint
	return system
}

func (planet *Planet) newAPI(count int, identity *identity.FullIdentity, config satellite.Config, versionInfo version.Info) (*satellite.API, error) {
	prefix := "satellite-api" + strconv.Itoa(count)
	log := planet.log.Named(prefix)
	var err error

	revocationDB, err := revocation.NewDBFromCfg(config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	var db satellite.DB
	if planet.config.Reconfigure.NewSatelliteDB != nil {
		db, err = planet.config.Reconfigure.NewSatelliteDB(log.Named("db"), count)
	} else {
		db, err = satellitedb.NewInMemory(log.Named("db"))
	}
	if err != nil {
		return nil, err
	}
	planet.databases = append(planet.databases, db)

	return satellite.NewAPI(log, identity, db, revocationDB, &config, versionInfo)
}
