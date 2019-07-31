// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
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
	"storj.io/storj/satellite/vouchers"
)

// newSatellites initializes satellites
func (planet *Planet) newSatellites(count int) ([]*satellite.Peer, error) {
	var xs []*satellite.Peer
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

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}

		planet.databases = append(planet.databases, db)

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
			Kademlia: kademlia.Config{
				Alpha:                5,
				BootstrapBackoffBase: 500 * time.Millisecond,
				BootstrapBackoffMax:  2 * time.Second,
				DBPath:               storageDir, // TODO: replace with master db
				Operator: kademlia.OperatorConfig{
					Email:  prefix + "@mail.test",
					Wallet: "0x" + strings.Repeat("00", 20),
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
				DiscoveryInterval:  1 * time.Second,
				RefreshInterval:    1 * time.Second,
				RefreshLimit:       100,
				RefreshConcurrency: 2,
			},
			Metainfo: metainfo.Config{
				DatabaseURL:          "bolt://" + filepath.Join(storageDir, "pointers.db"),
				MinRemoteSegmentSize: 0, // TODO: fix tests to work with 1024
				MaxInlineSegmentSize: 8000,
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
					CoalesceDuration: 5 * time.Second,
				},
			},
			Orders: orders.Config{
				Expiration: 7 * 24 * time.Hour,
			},
			Checker: checker.Config{
				Interval:                  30 * time.Second,
				IrreparableInterval:       15 * time.Second,
				ReliabilityCacheStaleness: 5 * time.Minute,
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
				Interval:           30 * time.Second,
				MinBytesPerSecond:  1 * memory.KB,
				MinDownloadTimeout: 5 * time.Second,
			},
			GarbageCollection: gc.Config{
				Interval:          1 * time.Minute,
				Enabled:           true,
				InitialPieces:     10,
				FalsePositiveRate: 0.1,
				ConcurrentSends:   1,
			},
			Tally: tally.Config{
				Interval: 30 * time.Second,
			},
			Rollup: rollup.Config{
				Interval:      2 * time.Minute,
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
			Vouchers: vouchers.Config{
				Expiration: 30 * 24 * time.Hour,
			},
			Version: planet.NewVersionConfig(),
		}
		if planet.config.Reconfigure.Satellite != nil {
			planet.config.Reconfigure.Satellite(log, i, &config)
		}

		verInfo := planet.NewVersionInfo()

		peer, err := satellite.New(log, identity, db, &config, verInfo)
		if err != nil {
			return xs, err
		}

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())
		xs = append(xs, peer)
	}
	return xs, nil
}
