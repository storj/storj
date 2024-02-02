// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/memory"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/storj/cmd/storagenode/internalcmd"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/forgetsatellite"
	"storj.io/storj/storagenode/gracefulexit"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/preflight"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
	"storj.io/storj/storagenode/version"
)

// StorageNode contains all the processes needed to run a full StorageNode setup.
type StorageNode struct {
	Name   string
	Config storagenode.Config
	*storagenode.Peer

	apiKey apikeys.APIKey
}

// Label returns name for debugger.
func (system *StorageNode) Label() string { return system.Name }

// URL returns the node url as a string.
func (system *StorageNode) URL() string { return system.NodeURL().String() }

// NodeURL returns the storj.NodeURL.
func (system *StorageNode) NodeURL() storj.NodeURL {
	return storj.NodeURL{ID: system.Peer.ID(), Address: system.Peer.Addr()}
}

// APIKey returns the API key of the node.
func (system *StorageNode) APIKey() string {
	return system.apiKey.Secret.String()
}

// newStorageNodes initializes storage nodes.
func (planet *Planet) newStorageNodes(ctx context.Context, count int, whitelistedSatellites storj.NodeURLs) (_ []*StorageNode, err error) {
	defer mon.Task()(&ctx)(&err)

	var sources []trust.Source
	for _, u := range whitelistedSatellites {
		source, err := trust.NewStaticURLSource(u.String())
		if err != nil {
			return nil, errs.Wrap(err)
		}
		sources = append(sources, source)
	}

	var xs []*StorageNode
	for i := 0; i < count; i++ {
		index := i
		prefix := "storage" + strconv.Itoa(index)
		log := planet.log.Named(prefix)

		var system *StorageNode
		var err error
		pprof.Do(ctx, pprof.Labels("peer", prefix), func(ctx context.Context) {
			system, err = planet.newStorageNode(ctx, prefix, index, count, log, sources)
		})
		if err != nil {
			return nil, errs.Wrap(err)
		}

		log.Debug("id=" + system.ID().String() + " addr=" + system.Addr())
		xs = append(xs, system)
		planet.peers = append(planet.peers, newClosablePeer(system))
	}
	return xs, nil
}

func (planet *Planet) newStorageNode(ctx context.Context, prefix string, index, count int, log *zap.Logger, sources []trust.Source) (_ *StorageNode, err error) {
	defer mon.Task()(&ctx)(&err)

	storageDir := filepath.Join(planet.directory, prefix)
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, errs.Wrap(err)
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	config := storagenode.Config{
		Server: server.Config{
			Address:        planet.NewListenAddress(),
			PrivateAddress: planet.NewListenAddress(),

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
		Debug: debug.Config{
			Addr: "",
		},
		Preflight: preflight.Config{
			LocalTimeCheck: false,
		},
		Operator: operator.Config{
			Email:          prefix + "@mail.test",
			Wallet:         "0x" + strings.Repeat("00", 20),
			WalletFeatures: nil,
		},
		Storage: piecestore.OldConfig{
			Path:                   filepath.Join(storageDir, "pieces/"),
			AllocatedDiskSpace:     1 * memory.GB,
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
			Address:   planet.NewListenAddress(),
			StaticDir: filepath.Join(developmentRoot, "web/storagenode/"),
		},
		Storage2: piecestore.Config{
			CacheSyncInterval:       defaultInterval,
			ExpirationGracePeriod:   0,
			MaxConcurrentRequests:   100,
			OrderLimitGracePeriod:   time.Hour,
			StreamOperationTimeout:  time.Hour,
			ReportCapacityThreshold: 100 * memory.MB,
			DeleteQueueSize:         10000,
			DeleteWorkers:           1,
			ExistsCheckWorkers:      5,
			Orders: orders.Config{
				SenderInterval:  defaultInterval,
				SenderTimeout:   10 * time.Minute,
				CleanupInterval: defaultInterval,
				ArchiveTTL:      time.Hour,
				MaxSleep:        0,
				Path:            filepath.Join(storageDir, "orders"),
			},
			Monitor: monitor.Config{
				MinimumDiskSpace:          100 * memory.MB,
				NotifyLowDiskCooldown:     defaultInterval,
				VerifyDirReadableInterval: defaultInterval,
				VerifyDirWritableInterval: defaultInterval,
				VerifyDirReadableTimeout:  10 * time.Second,
				VerifyDirWritableTimeout:  10 * time.Second,
			},
			Trust: trust.Config{
				Sources:         sources,
				CachePath:       filepath.Join(storageDir, "trust-cache.json"),
				RefreshInterval: defaultInterval,
			},
			MaxUsedSerialsSize: memory.MiB,
		},
		Pieces:    pieces.DefaultConfig,
		Filestore: filestore.DefaultConfig,
		Retain: retain.Config{
			MaxTimeSkew: 10 * time.Second,
			Status:      retain.Enabled,
			Concurrency: 5,
		},
		Version: version.Config{
			Config: planet.NewVersionConfig(),
		},
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
		ForgetSatellite: forgetsatellite.Config{
			ChoreInterval: defaultInterval,
			NumWorkers:    3,
		},
	}

	if os.Getenv("STORJ_TEST_DISABLEQUIC") != "" {
		config.Server.DisableQUIC = true
	}

	// enable the lazy filewalker
	config.Pieces.EnableLazyFilewalker = true

	if planet.config.Reconfigure.StorageNode != nil {
		planet.config.Reconfigure.StorageNode(index, &config)
	}

	newIPCount := planet.config.Reconfigure.UniqueIPCount
	if newIPCount > 0 {
		if index >= count-newIPCount {
			config.Server.Address = fmt.Sprintf("127.0.%d.1:0", index+1)
			config.Server.PrivateAddress = fmt.Sprintf("127.0.%d.1:0", index+1)
		}
	}

	verisonInfo := planet.NewVersionInfo()

	dbconfig := config.DatabaseConfig()
	dbconfig.TestingDisableWAL = true
	var db storagenode.DB
	db, err = storagenodedbtest.OpenNew(ctx, log.Named("db"), config.DatabaseConfig())
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if err := db.Pieces().CreateVerificationFile(ctx, identity.ID); err != nil {
		return nil, errs.Wrap(err)
	}

	if planet.config.Reconfigure.StorageNodeDB != nil {
		db, err = planet.config.Reconfigure.StorageNodeDB(index, db, planet.log)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	peer, err := storagenode.New(log, identity, db, revocationDB, config, verisonInfo, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Mark the peer's PieceDeleter as in testing mode, so it is easy to wait on the deleter
	peer.Storage2.PieceDeleter.SetupTest()

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, db)

	service := apikeys.NewService(db.APIKeys())

	apiKey, err := service.Issue(ctx)
	if err != nil {
		return nil, errs.New("error while trying to issue new api key: %v", err)
	}

	{
		// set up the used space lazyfilewalker filewalker
		cmd := internalcmd.NewUsedSpaceFilewalkerCmd()
		cmd.Logger = log.Named("used-space-filewalker")
		cmd.Ctx = ctx
		peer.Storage2.LazyFileWalker.TestingSetUsedSpaceCmd(cmd)
	}
	{
		// set up the GC lazyfilewalker filewalker
		cmd := internalcmd.NewGCFilewalkerCmd()
		cmd.Logger = log.Named("gc-filewalker")
		cmd.Ctx = ctx
		peer.Storage2.LazyFileWalker.TestingSetGCCmd(cmd)
	}
	{
		// set up the trash cleanup lazyfilewalker filewalker
		cmd := internalcmd.NewTrashFilewalkerCmd()
		cmd.Logger = log.Named("trash-filewalker")
		cmd.Ctx = ctx
		peer.Storage2.LazyFileWalker.TestingSetTrashCleanupCmd(cmd)
	}

	return &StorageNode{
		Name:   prefix,
		Config: config,
		Peer:   peer,
		apiKey: apiKey,
	}, nil
}
