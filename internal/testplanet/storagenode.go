// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb"
)

// newStorageNodes initializes storage nodes
func (planet *Planet) newStorageNodes(count int, whitelistedSatellites storj.NodeURLs) ([]*storagenode.Peer, error) {
	var xs []*storagenode.Peer
	defer func() {
		for _, x := range xs {
			planet.peers = append(planet.peers, closablePeer{peer: x})
		}
	}()

	for i := 0; i < count; i++ {
		prefix := "storage" + strconv.Itoa(i)
		log := planet.log.Named(prefix)
		storageDir := filepath.Join(planet.directory, prefix)

		if err := os.MkdirAll(storageDir, 0700); err != nil {
			return nil, err
		}

		identity, err := planet.NewIdentity()
		if err != nil {
			return nil, err
		}

		config := storagenode.Config{
			Server: server.Config{
				Address:        "127.0.0.1:0",
				PrivateAddress: "127.0.0.1:0",

				Config: tlsopts.Config{
					RevocationDBURL:     "bolt://" + filepath.Join(storageDir, "revocation.db"),
					UsePeerCAWhitelist:  true,
					PeerCAWhitelistPath: planet.whitelistPath,
					PeerIDVersions:      "*",
					Extensions: extensions.Config{
						Revocation:          false,
						WhitelistSignedLeaf: false,
					},
				},
			},
			Kademlia: kademlia.Config{
				Operator: kademlia.OperatorConfig{
					Email:  prefix + "@mail.test",
					Wallet: "0x" + strings.Repeat("00", 20),
				},
			},
			Storage: piecestore.OldConfig{
				Path:                   filepath.Join(storageDir, "pieces/"),
				AllocatedDiskSpace:     1 * memory.GB,
				AllocatedBandwidth:     memory.TB,
				KBucketRefreshInterval: time.Hour,
				WhitelistedSatellites:  whitelistedSatellites,
			},
			Collector: collector.Config{
				Interval: time.Minute,
			},
			Nodestats: nodestats.Config{
				MaxSleep:       0,
				ReputationSync: 1 * time.Minute,
				StorageSync:    1 * time.Minute,
			},
			Console: consoleserver.Config{
				Address:   "127.0.0.1:0",
				StaticDir: filepath.Join(developmentRoot, "web/storagenode/"),
			},
			Storage2: piecestore.Config{
				CacheSyncInterval:     time.Hour,
				ExpirationGracePeriod: 0,
				MaxConcurrentRequests: 100,
				OrderLimitGracePeriod: time.Hour,
				Orders: orders.Config{
					SenderInterval:  time.Hour,
					SenderTimeout:   time.Hour,
					CleanupInterval: time.Hour,
					ArchiveTTL:      time.Hour,
				},
				Monitor: monitor.Config{
					MinimumBandwidth: 100 * memory.MB,
					MinimumDiskSpace: 100 * memory.MB,
				},
			},
			Retain: retain.Config{
				Status:      retain.Enabled,
				Concurrency: 5,
			},
			Version: planet.NewVersionConfig(),
			Bandwidth: bandwidth.Config{
				Interval: time.Hour,
			},
			Contact: contact.Config{
				Interval: 30 * time.Second,
				MaxSleep: 0 * time.Second,
			},
		}
		if planet.config.Reconfigure.StorageNode != nil {
			planet.config.Reconfigure.StorageNode(i, &config)
		}

		newIPCount := planet.config.Reconfigure.NewIPCount
		if newIPCount > 0 {
			if i >= count-newIPCount {
				config.Server.Address = fmt.Sprintf("127.0.%d.1:0", i+1)
				config.Server.PrivateAddress = fmt.Sprintf("127.0.%d.1:0", i+1)
			}
		}

		verisonInfo := planet.NewVersionInfo()

		storageConfig := storagenodedb.Config{
			Storage: config.Storage.Path,
			Info:    filepath.Join(config.Storage.Path, "piecestore.db"),
			Info2:   filepath.Join(config.Storage.Path, "info.db"),
			Pieces:  config.Storage.Path,
		}

		var db storagenode.DB
		db, err = storagenodedb.New(log.Named("db"), storageConfig)
		if err != nil {
			return nil, err
		}

		if planet.config.Reconfigure.NewStorageNodeDB != nil {
			db, err = planet.config.Reconfigure.NewStorageNodeDB(i, db, planet.log)
			if err != nil {
				return nil, err
			}
		}

		revocationDB, err := revocation.NewDBFromCfg(config.Server.Config)
		if err != nil {
			return xs, errs.Wrap(err)
		}
		planet.databases = append(planet.databases, revocationDB)

		peer, err := storagenode.New(log, identity, db, revocationDB, config, verisonInfo)
		if err != nil {
			return xs, err
		}

		err = db.CreateTables()
		if err != nil {
			return nil, err
		}
		planet.databases = append(planet.databases, db)

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())
		xs = append(xs, peer)
	}
	return xs, nil
}
