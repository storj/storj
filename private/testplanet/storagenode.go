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

	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/storj/cmd/storagenode/internalcmd"
	"storj.io/storj/private/revocation"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/operator"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

// StorageNode contains all the processes needed to run a full StorageNode setup.
type StorageNode struct {
	Name   string
	Config storagenode.Config
	*storagenode.Peer
}

// Label returns name for debugger.
func (system *StorageNode) Label() string { return system.Name }

// URL returns the node url as a string.
func (system *StorageNode) URL() string { return system.NodeURL().String() }

// NodeURL returns the storj.NodeURL.
func (system *StorageNode) NodeURL() storj.NodeURL {
	return storj.NodeURL{ID: system.Peer.ID(), Address: system.Peer.Addr()}
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

	var config storagenode.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
		cfgstruct.ConfDir(storageDir),
		cfgstruct.IdentityDir(storageDir),
		cfgstruct.ConfigVar("TESTINTERVAL", defaultInterval.String()),
		cfgstruct.ConfigVar("HOST", planet.config.Host),
	)

	// TODO move some of those flags to the config files as 'testDefault'
	config.Server.Address = planet.NewListenAddress()
	config.Server.PrivateAddress = planet.NewListenAddress()
	config.Server.Config = tlsopts.Config{
		UsePeerCAWhitelist:  true,
		PeerCAWhitelistPath: planet.whitelistPath,
		PeerIDVersions:      "*",
		Extensions: extensions.Config{
			Revocation:          false,
			WhitelistSignedLeaf: false,
		},
	}
	config.Preflight.LocalTimeCheck = false
	config.Operator = operator.Config{
		Email:          prefix + "@mail.test",
		Wallet:         "0x" + strings.Repeat("00", 20),
		WalletFeatures: nil,
	}
	config.Storage.AllocatedDiskSpace = 1 * memory.GB
	config.Storage.KBucketRefreshInterval = defaultInterval

	config.Collector.Interval = defaultInterval

	config.Reputation.MaxSleep = 0
	config.Reputation.Interval = defaultInterval

	config.Nodestats.MaxSleep = 0
	config.Nodestats.StorageSync = defaultInterval

	config.Console = consoleserver.Config{
		Address:   planet.NewListenAddress(),
		StaticDir: filepath.Join(developmentRoot, "web/storagenode/"),
	}

	config.Storage2.CacheSyncInterval = defaultInterval
	config.Storage2.ExpirationGracePeriod = 0
	config.Storage2.MaxConcurrentRequests = 100
	config.Storage2.OrderLimitGracePeriod = time.Hour
	config.Storage2.StreamOperationTimeout = time.Hour
	config.Storage2.ReportCapacityThreshold = 100 * memory.MB

	config.Storage2.Orders.SenderInterval = defaultInterval
	config.Storage2.Orders.SenderTimeout = 10 * time.Minute
	config.Storage2.Orders.CleanupInterval = defaultInterval
	config.Storage2.Orders.ArchiveTTL = time.Hour
	config.Storage2.Orders.MaxSleep = 0

	config.Storage2.Monitor.Interval = defaultInterval
	config.Storage2.Monitor.MinimumDiskSpace = 100 * memory.MB
	config.Storage2.Monitor.NotifyLowDiskCooldown = defaultInterval
	config.Storage2.Monitor.VerifyDirReadableInterval = defaultInterval
	config.Storage2.Monitor.VerifyDirWritableInterval = defaultInterval

	config.Storage2.Trust.Sources = sources
	config.Storage2.Trust.RefreshInterval = defaultInterval

	config.Storage2Migration.BufferSize = 1
	config.Storage2Migration.Interval = defaultInterval

	config.Pieces = pieces.DefaultConfig
	config.Filestore = filestore.DefaultConfig

	config.Retain.MaxTimeSkew = 10 * time.Second
	config.Retain.CachePath = filepath.Join(planet.directory, "retain")

	config.Version.Config = planet.NewVersionConfig()

	config.Bandwidth.Interval = defaultInterval

	config.Contact.Interval = defaultInterval
	config.Contact.CheckInTimeout = 15 * time.Second

	config.ForgetSatellite.NumWorkers = 3

	if os.Getenv("STORJ_TEST_DISABLEQUIC") != "" {
		config.Server.DisableQUIC = true
	}

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
	db, err = storagenodedbtest.OpenNew(ctx, log.Named("db"), dbconfig)
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

	// enable the Testing methdos to delete/corrupt pieces.
	peer.Storage2.PieceBackend.TestingEnableMethods()
	thePast := time.Now().AddDate(-1, 0, 0)

	// flag some of the storage nodes to be in different migration states.
	for _, trustSource := range sources {
		entries, err := trustSource.FetchEntries(ctx)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		for _, entry := range entries {
			// set the restore time for the node to be way in the past.
			if err := peer.Storage2.RestoreTimeManager.SetRestoreTime(ctx, entry.SatelliteURL.ID, thePast); err != nil {
				return nil, errs.Wrap(err)
			}

			// set the node to write (+ TTL)/read to/from new sometimes.
			// migrate actively and passively sometimes.
			//
			// this is how the different parameters alternate:
			//
			//                  |           11111111112222…
			// number of nodes  | 012345678901234567890123…
			// --------------------------------------------
			// TTLToNew         | -x-x-x-x-x-x-x-x-x-x-x-x…
			// WriteToNew       | x-x-x-x-x-x-x-x-x-x-x-x-…
			// ReadNewFirst     | xx--xx--xx--xx--xx--xx--…
			// PassiveMigrate   | xxxx----xxxx----xxxx----…
			// active migration | xxxxxxxx--------xxxxxxxx…
			peer.Storage2.MigratingBackend.UpdateState(ctx, entry.SatelliteURL.ID, func(state *piecestore.MigrationState) {
				state.TTLToNew = index%2 != 0
				state.WriteToNew = index%2 == 0
				state.ReadNewFirst = index%4 < 2
				state.PassiveMigrate = index%8 < 4
			})
			peer.Storage2.MigrationChore.SetMigrate(entry.SatelliteURL.ID, true, index%16 < 8)
		}
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, db)

	if config.Pieces.EnableLazyFilewalker {
		{
			// set up the used space lazyfilewalker filewalker
			cmd := internalcmd.NewUsedSpaceFilewalkerCmd()
			cmd.Logger = log.Named("used-space-filewalker")
			cmd.Ctx = ctx
			peer.StorageOld.LazyFileWalker.TestingSetUsedSpaceCmd(cmd)
		}
		{
			// set up the GC lazyfilewalker filewalker
			cmd := internalcmd.NewGCFilewalkerCmd()
			cmd.Logger = log.Named("gc-filewalker")
			cmd.Ctx = ctx
			peer.StorageOld.LazyFileWalker.TestingSetGCCmd(cmd)
		}
		{
			// set up the trash cleanup lazyfilewalker filewalker
			cmd := internalcmd.NewTrashFilewalkerCmd()
			cmd.Logger = log.Named("trash-filewalker")
			cmd.Ctx = ctx
			peer.StorageOld.LazyFileWalker.TestingSetTrashCleanupCmd(cmd)
		}
	}

	return &StorageNode{
		Name:   prefix,
		Config: config,
		Peer:   peer,
	}, nil
}
