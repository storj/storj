// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/trust"
)

// newStorageNodes initializes storage nodes
func (planet *Planet) newStorageNodes(count int, whitelistedSatellites storj.NodeURLs) ([]*storagenode.Peer, error) {
	var xs []*storagenode.Peer
	defer func() {
		for _, x := range xs {
			planet.peers = append(planet.peers, newClosablePeer(x))
		}
	}()

	var sources []trust.Source
	for _, u := range whitelistedSatellites {
		source, err := trust.NewStaticURLSource(u.String())
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

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
			Operator: storagenode.OperatorConfig{
				Email:  prefix + "@mail.test",
				Wallet: "0x" + strings.Repeat("00", 20),
			},
			Storage: piecestore.OldConfig{
				Path:                   filepath.Join(storageDir, "pieces/"),
				AllocatedDiskSpace:     1 * memory.GB,
				AllocatedBandwidth:     memory.TB,
				KBucketRefreshInterval: defaultInterval,
			},
			Collector: collector.Config{
				Interval: defaultInterval,
			},
			Nodestats: nodestats.Config{
				MaxSleep:       0,
				ReputationSync: defaultInterval,
				StorageSync:    defaultInterval,
			},
			Console: consoleserver.Config{
				Address:   "127.0.0.1:0",
				StaticDir: filepath.Join(developmentRoot, "web/storagenode/"),
			},
			Storage2: piecestore.Config{
				CacheSyncInterval:     defaultInterval,
				ExpirationGracePeriod: 0,
				MaxConcurrentRequests: 100,
				OrderLimitGracePeriod: time.Hour,
				Orders: orders.Config{
					SenderInterval:  defaultInterval,
					SenderTimeout:   10 * time.Minute,
					CleanupInterval: defaultInterval,
					ArchiveTTL:      time.Hour,
				},
				Monitor: monitor.Config{
					MinimumBandwidth: 100 * memory.MB,
					MinimumDiskSpace: 100 * memory.MB,
				},
				Trust: trust.Config{
					Sources:         sources,
					CachePath:       filepath.Join(storageDir, "trust-cache.json"),
					RefreshInterval: defaultInterval,
				},
			},
			Retain: retain.Config{
				Status:      retain.Enabled,
				Concurrency: 5,
			},
			Version: planet.NewVersionConfig(),
			Bandwidth: bandwidth.Config{
				Interval: defaultInterval,
			},
			Contact: contact.Config{
				Interval: defaultInterval,
			},
			GracefulExit: gracefulexit.Config{
				ChoreInterval:          defaultInterval,
				NumWorkers:             3,
				NumConcurrentTransfers: 1,
				MinBytesPerSecond:      128 * memory.B,
				MinDownloadTimeout:     2 * time.Minute,
			},
		}
		if planet.config.Reconfigure.StorageNode != nil {
			planet.config.Reconfigure.StorageNode(i, &config)
		}

		newIPCount := planet.config.Reconfigure.UniqueIPCount
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

		err = db.CreateTables(context.TODO())
		if err != nil {
			return nil, err
		}
		planet.databases = append(planet.databases, db)

		log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())
		xs = append(xs, peer)
	}
	return xs, nil
}
