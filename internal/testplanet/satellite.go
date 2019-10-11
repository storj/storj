// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
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
	"storj.io/storj/satellite/gc"
	"storj.io/storj/satellite/gracefulexit"
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

		var pointerDB metainfo.PointerDB
		if planet.config.Reconfigure.NewSatellitePointerDB != nil {
			pointerDB, err = planet.config.Reconfigure.NewSatellitePointerDB(log.Named("pointerdb"), i)
		} else {
			pointerDB, err = metainfo.NewStore(log.Named("pointerdb"), "bolt://"+filepath.Join(storageDir, "pointers.db"))
		}
		if err != nil {
			return nil, err
		}

		config := satellite.Config{
			Server: server.Config{
				Address:        "127.0.0.1:0",
				PrivateAddress: "127.0.0.1:0",

				Config: tlsopts.Config{
					RevocationDBURL:     "bolt://" + filepath.Join(storageDir, "revocation.db"),
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
			Metainfo: metainfo.Config{
				DatabaseURL:          "", // not used
				MinRemoteSegmentSize: 0,  // TODO: fix tests to work with 1024
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
			GracefulExit: gracefulexit.Config{
				ChoreBatchSize: 10,
				ChoreInterval:  defaultInterval,

				EndpointBatchSize:   100,
				EndpointMaxFailures: 5,
			},
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

		peer, err := satellite.New(log, identity, db, pointerDB, revocationDB, versionInfo, &config)
		if err != nil {
			return xs, err
		}

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}
		planet.databases = append(planet.databases, db)
		planet.databases = append(planet.databases, pointerDB)

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())

		system := SatelliteSystem{Peer: *peer}
		xs = append(xs, &system)
	}
	return xs, nil
}
