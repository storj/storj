// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/cfgstruct"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/macaroon"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/common/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/private/server"
	"storj.io/storj/private/testredis"
	versionchecker "storj.io/storj/private/version/checker"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/userinfo"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/gc/sender"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/nodestats"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/offlinenodes"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/lrucache"
)

// Satellite contains all the processes needed to run a full Satellite setup.
type Satellite struct {
	Name   string
	Config satellite.Config

	Core       *satellite.Core
	API        *satellite.API
	ConsoleAPI *satellite.ConsoleAPI
	UI         *satellite.UI
	Repairer   *satellite.Repairer
	Auditor    *satellite.Auditor
	Admin      *satellite.Admin
	GCBF       *satellite.GarbageCollectionBF
	RangedLoop *satellite.RangedLoop

	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       satellite.DB

	Dialer rpc.Dialer

	Server *server.Server

	Version *versionchecker.Service

	Contact struct {
		Service  *contact.Service
		Endpoint *contact.Endpoint
	}

	Overlay struct {
		DB                overlay.DB
		Service           *overlay.Service
		OfflineNodeEmails *offlinenodes.Chore
		DQStrayNodes      *straynodes.Chore
	}

	NodeEvents struct {
		DB       nodeevents.DB
		Notifier nodeevents.Notifier
		Chore    *nodeevents.Chore
	}

	Metainfo struct {
		Endpoint *metainfo.Endpoint
	}

	Userinfo struct {
		Endpoint *userinfo.Endpoint
	}

	Metabase struct {
		DB *metabase.DB
	}

	Orders struct {
		DB       orders.DB
		Endpoint *orders.Endpoint
		Service  *orders.Service
		Chore    *orders.Chore
	}

	Repair struct {
		Repairer *repairer.Service
		Queue    queue.RepairQueue
	}

	Audit struct {
		VerifyQueue          audit.VerifyQueue
		ReverifyQueue        audit.ReverifyQueue
		Worker               *audit.Worker
		ReverifyWorker       *audit.ReverifyWorker
		Verifier             *audit.Verifier
		Reverifier           *audit.Reverifier
		Reporter             audit.Reporter
		ContainmentSyncChore *audit.ContainmentSyncChore
	}

	Reputation struct {
		Service *reputation.Service
	}

	GarbageCollection struct {
		Sender *sender.Service
	}

	ExpiredDeletion struct {
		Chore *expireddeletion.Chore
	}

	ZombieDeletion struct {
		Chore *zombiedeletion.Chore
	}

	Accounting struct {
		Tally            *tally.Service
		Rollup           *rollup.Service
		ProjectUsage     *accounting.Service
		ProjectBWCleanup *projectbwcleanup.Chore
		RollupArchive    *rolluparchive.Chore
	}

	LiveAccounting struct {
		Cache accounting.Cache
	}

	Mail struct {
		Service *mailservice.Service
	}

	ConsoleFrontend struct {
		Listener net.Listener
		Endpoint *consoleweb.Server
	}

	NodeStats struct {
		Endpoint *nodestats.Endpoint
	}

	GracefulExit struct {
		Endpoint *gracefulexit.Endpoint
	}
}

// Label returns name for debugger.
func (system *Satellite) Label() string { return system.Name }

// ID returns the ID of the Satellite system.
func (system *Satellite) ID() storj.NodeID { return system.API.Identity.ID }

// Addr returns the public address from the Satellite system API.
func (system *Satellite) Addr() string { return system.API.Server.Addr().String() }

// URL returns the node url from the Satellite system API.
func (system *Satellite) URL() string { return system.NodeURL().String() }

// ConsoleURL returns the console URL.
func (system *Satellite) ConsoleURL() string {
	if system.Config.DisableConsoleFromSatelliteAPI {
		return "http://" + system.ConsoleAPI.Console.Listener.Addr().String()
	} else {
		return "http://" + system.API.Console.Listener.Addr().String()
	}
}

// NodeURL returns the storj.NodeURL from the Satellite system API.
func (system *Satellite) NodeURL() storj.NodeURL {
	return storj.NodeURL{ID: system.API.ID(), Address: system.API.Addr()}
}

// AddUser adds user to a satellite. Password from newUser will be always overridden by FullName to have
// known password which can be used automatically.
func (system *Satellite) AddUser(ctx context.Context, newUser console.CreateUser, maxNumberOfProjects int) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	var service *console.Service
	if system.Config.DisableConsoleFromSatelliteAPI && system.ConsoleAPI != nil {
		service = system.ConsoleAPI.Console.Service
	} else {
		service = system.API.Console.Service
	}

	var regTokenSecret console.RegistrationSecret
	if !system.Config.Console.OpenRegistrationEnabled {
		regToken, err := service.CreateRegToken(ctx, maxNumberOfProjects)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		regTokenSecret = regToken.Secret
	}

	newUser.Password = newUser.FullName
	user, err := service.CreateUser(ctx, newUser, regTokenSecret)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if err = service.SetAccountActive(ctx, user); err != nil {
		return nil, errs.Wrap(err)
	}

	userCtx := console.WithUser(ctx, user)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	_, err = service.Payments().SetupAccount(userCtx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return user, nil
}

// AddProject adds project to a satellite and makes specified user an owner.
// This method only mimics the behavior of the console API. It's simplified to
// be faster for tests.
func (system *Satellite) AddProject(ctx context.Context, ownerID uuid.UUID, name string) (project *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err = system.DB.Console().Projects().Insert(ctx, &console.Project{
		ID:             testrand.UUID(),
		PublicID:       testrand.UUID(),
		Name:           name,
		OwnerID:        ownerID,
		StorageLimit:   &system.Config.Console.UsageLimits.Storage.Free,
		BandwidthLimit: &system.Config.Console.UsageLimits.Bandwidth.Free,
		SegmentLimit:   &system.Config.Console.UsageLimits.Segment.Free,
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	_, err = system.DB.Console().ProjectMembers().Insert(ctx, ownerID, project.ID, console.RoleAdmin)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return project, nil
}

// CreateAPIKey creates an API key for the specified project and user with the given version.
// This method only mimics the behavior of the console API. It's simplified to
// be faster for tests.
func (system *Satellite) CreateAPIKey(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, version macaroon.APIKeyVersion) (_ *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := macaroon.NewSecret()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	key, err := macaroon.NewAPIKey(secret)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	apikey := console.APIKeyInfo{
		Name:      "root",
		ProjectID: projectID,
		CreatedBy: userID,
		Secret:    secret,
		Version:   version,
	}

	_, err = system.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return key, nil
}

// UserContext creates context with user.
func (system *Satellite) UserContext(ctx context.Context, userID uuid.UUID) (_ context.Context, err error) {
	defer mon.Task()(&ctx)(&err)

	var user *console.User
	if system.Config.DisableConsoleFromSatelliteAPI && system.ConsoleAPI != nil {
		user, err = system.ConsoleAPI.Console.Service.GetUser(ctx, userID)
	} else {
		user, err = system.API.Console.Service.GetUser(ctx, userID)
	}
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return console.WithUser(ctx, user), nil
}

// Close closes all the subsystems in the Satellite system.
func (system *Satellite) Close() (err error) {
	err = errs.Combine(
		system.API.Close(),
		system.Core.Close(),
		system.Repairer.Close(),
		system.Auditor.Close(),
		system.Admin.Close(),
		system.GCBF.Close(),
	)
	if system.ConsoleAPI != nil {
		err = errs.Combine(err, system.ConsoleAPI.Close())
	}

	return err
}

// Run runs all the subsystems in the Satellite system.
func (system *Satellite) Run(ctx context.Context) (err error) {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(system.Core.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.API.Run(ctx))
	})
	if system.ConsoleAPI != nil {
		group.Go(func() error {
			return errs2.IgnoreCanceled(system.ConsoleAPI.Run(ctx))
		})
	}
	if system.UI != nil {
		group.Go(func() error {
			return errs2.IgnoreCanceled(system.UI.Run(ctx))
		})
	}
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.Repairer.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.Auditor.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.Admin.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.GCBF.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(system.RangedLoop.Run(ctx))
	})
	return group.Wait()
}

// PrivateAddr returns the private address from the Satellite system API.
func (system *Satellite) PrivateAddr() string { return system.API.Server.PrivateAddr().String() }

// newSatellites initializes satellites.
func (planet *Planet) newSatellites(ctx context.Context, count int, databases satellitedbtest.SatelliteDatabases) (_ []*Satellite, err error) {
	defer mon.Task()(&ctx)(&err)

	var satellites []*Satellite

	for i := 0; i < count; i++ {
		index := i
		prefix := "satellite" + strconv.Itoa(index)
		log := planet.log.Named(prefix)

		var system *Satellite
		var err error

		pprof.Do(ctx, pprof.Labels("peer", prefix), func(ctx context.Context) {
			system, err = planet.newSatellite(ctx, prefix, index, log, databases, planet.config.applicationName)
		})
		if err != nil {
			return nil, errs.Wrap(err)
		}

		log.Debug("id=" + system.ID().String() + " addr=" + system.Addr())
		satellites = append(satellites, system)
		planet.peers = append(planet.peers, newClosablePeer(system))
	}

	return satellites, nil
}

func (planet *Planet) newSatellite(ctx context.Context, prefix string, index int, log *zap.Logger, databases satellitedbtest.SatelliteDatabases, applicationName string) (_ *Satellite, err error) {
	defer mon.Task()(&ctx)(&err)

	storageDir := filepath.Join(planet.directory, prefix)
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, errs.Wrap(err)
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	defaultSatDBOptions := satellitedb.Options{
		ApplicationName: applicationName,
		APIKeysLRUOptions: lrucache.Options{
			Expiration: 1 * time.Minute,
			Capacity:   100,
		},
		RevocationLRUOptions: lrucache.Options{
			Expiration: 1 * time.Minute,
			Capacity:   100,
		},
	}
	if planet.config.Reconfigure.SatelliteDBOptions != nil {
		planet.config.Reconfigure.SatelliteDBOptions(log, index, &defaultSatDBOptions)
	}

	db, err := satellitedbtest.CreateMasterDB(ctx, log.Named("db"), planet.config.Name, "S", index, databases.MasterDB, defaultSatDBOptions)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if planet.config.Reconfigure.SatelliteDB != nil {
		var newdb satellite.DB
		newdb, err = planet.config.Reconfigure.SatelliteDB(log.Named("db"), index, db)
		if err != nil {
			return nil, errs.Combine(err, db.Close())
		}
		db = newdb
	}
	planet.databases = append(planet.databases, db)

	redis, err := testredis.Mini(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	encryptionKeys, err := orders.NewEncryptionKeys(orders.EncryptionKey{
		ID:  orders.EncryptionKeyID{1},
		Key: storj.Key{1},
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var config satellite.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
		cfgstruct.ConfDir(storageDir),
		cfgstruct.IdentityDir(storageDir),
		cfgstruct.ConfigVar("TESTINTERVAL", defaultInterval.String()),
		cfgstruct.ConfigVar("HOST", planet.config.Host),
	)

	// TODO: these are almost certainly mistakenly set to the zero value
	// in tests due to a prior mismatch between testplanet config and
	// cfgstruct devDefaults. we need to make sure it's safe to remove
	// these lines and then remove them.
	config.Debug.Control = false
	config.Debug.Addr = ""
	config.Reputation.AuditHistory.OfflineDQEnabled = false
	config.Server.Config.Extensions.Revocation = false
	config.Checker.NodeFailureRate = 0
	config.Audit.MaxRetriesStatDB = 0
	config.GarbageCollection.RetainSendTimeout = 0
	config.ExpiredDeletion.ListLimit = 0
	config.Tally.SaveRollupBatchSize = 0
	config.Tally.ReadRollupBatchSize = 0
	config.Rollup.DeleteTallies = false
	config.Payments.BonusRate = 0
	config.Identity.CertPath = ""
	config.Identity.KeyPath = ""
	config.Metainfo.DatabaseURL = ""
	config.Console.ContactInfoURL = ""
	config.Console.FrameAncestors = ""
	config.Console.LetUsKnowURL = ""
	config.Console.SEO = ""
	config.Console.SatelliteOperator = ""
	config.Console.TermsAndConditionsURL = ""
	config.Console.GeneralRequestURL = ""
	config.Console.ProjectLimitsIncreaseRequestURL = ""
	config.Console.GatewayCredentialsRequestURL = ""
	config.Console.DocumentationURL = ""
	config.Console.PathwayOverviewEnabled = false
	config.Compensation.Rates.AtRestGBHours = compensation.Rate{}
	config.Compensation.Rates.GetTB = compensation.Rate{}
	config.Compensation.Rates.GetRepairTB = compensation.Rate{}
	config.Compensation.Rates.GetAuditTB = compensation.Rate{}
	config.Compensation.WithheldPercents = nil
	config.Compensation.DisposePercent = 0

	// Actual testplanet-specific configuration
	config.Server.Address = planet.NewListenAddress()
	config.Server.PrivateAddress = planet.NewListenAddress()
	config.Admin.Address = planet.NewListenAddress()
	config.Console.Address = planet.NewListenAddress()
	config.Server.Config.PeerCAWhitelistPath = planet.whitelistPath
	config.Server.Config.UsePeerCAWhitelist = true
	config.Version = planet.NewVersionConfig()
	config.Metainfo.RS.Min = atLeastOne(planet.config.StorageNodeCount * 1 / 5)
	config.Metainfo.RS.Repair = atLeastOne(planet.config.StorageNodeCount * 2 / 5)
	config.Metainfo.RS.Success = atLeastOne(planet.config.StorageNodeCount * 3 / 5)
	config.Metainfo.RS.Total = atLeastOne(planet.config.StorageNodeCount * 4 / 5)
	config.Orders.EncryptionKeys = *encryptionKeys
	config.LiveAccounting.StorageBackend = "redis://" + redis.Addr() + "?db=0"
	config.Mail.TemplatePath = filepath.Join(developmentRoot, "web/satellite/static/emails")
	config.Console.StaticDir = filepath.Join(developmentRoot, "web/satellite")
	config.Payments.Storjscan.DisableLoop = true

	if os.Getenv("STORJ_TEST_DISABLEQUIC") != "" {
		config.Server.DisableQUIC = true
	}

	if planet.config.Reconfigure.Satellite != nil {
		planet.config.Reconfigure.Satellite(log, index, &config)
	}

	metabaseConfig := config.Metainfo.Metabase("satellite-testplanet")
	if planet.config.Reconfigure.SatelliteMetabaseDBConfig != nil {
		planet.config.Reconfigure.SatelliteMetabaseDBConfig(log, index, &metabaseConfig)
	}

	metabaseDB, err := satellitedbtest.CreateMetabaseDB(ctx, log.Named("metabase"), planet.config.Name, "M", index, databases.MetabaseDB,
		metabaseConfig)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if planet.config.Reconfigure.SatelliteMetabaseDB != nil {
		var newMetabaseDB *metabase.DB
		newMetabaseDB, err = planet.config.Reconfigure.SatelliteMetabaseDB(log.Named("metabase"), index, metabaseDB)
		if err != nil {
			return nil, errs.Combine(err, metabaseDB.Close())
		}
		metabaseDB = newMetabaseDB
	}
	planet.databases = append(planet.databases, metabaseDB)

	versionInfo := planet.NewVersionInfo()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var repairQueue queue.RepairQueue
	if !config.JobQueue.ServerNodeURL.IsZero() {
		repairQueue, err = jobq.OpenJobQueue(ctx, nil, config.JobQueue)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	} else {
		repairQueue = db.RepairQueue()
	}

	planet.databases = append(planet.databases, revocationDB)

	liveAccounting, err := live.OpenCache(ctx, log.Named("live-accounting"), config.LiveAccounting)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, liveAccounting)

	config.Payments.Provider = "mock"
	config.Payments.MockProvider = stripe.NewStripeMock(db.StripeCoinPayments().Customers(), db.Console().Users())

	peer, err := satellite.New(log, identity, db, metabaseDB, revocationDB, repairQueue, liveAccounting, versionInfo, &config, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if planet.config.LastNetFunc != nil {
		peer.Overlay.Service.LastNetFunc = planet.config.LastNetFunc
	}

	err = db.Testing().TestMigrateToLatest(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	err = metabaseDB.TestMigrateToLatest(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	api, err := planet.newAPI(ctx, index, identity, db, metabaseDB, config, versionInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var (
		consoleAPI     *satellite.ConsoleAPI
		consoleAPIAddr string
	)
	if config.DisableConsoleFromSatelliteAPI {
		consoleAPI, err = planet.newConsoleAPI(ctx, index, identity, db, metabaseDB, config, versionInfo)
		if err != nil {
			return nil, errs.Wrap(err)
		}

		consoleAPIAddr = consoleAPI.Console.Listener.Addr().String()
	} else {
		consoleAPIAddr = api.Console.Listener.Addr().String()
	}

	// only run if front-end endpoints on console back-end server are disabled.
	var ui *satellite.UI
	if !config.Console.FrontendEnable {
		ui, err = planet.newUI(ctx, index, identity, config, api.ExternalAddress, consoleAPIAddr)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	adminPeer, err := planet.newAdmin(ctx, index, identity, db, metabaseDB, config, versionInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	repairerPeer, err := planet.newRepairer(ctx, index, identity, db, metabaseDB, repairQueue, config, versionInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	auditorPeer, err := planet.newAuditor(ctx, index, identity, db, metabaseDB, config, versionInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	gcBFPeer, err := planet.newGarbageCollectionBF(ctx, index, db, metabaseDB, config, versionInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	rangedLoopPeer, err := planet.newRangedLoop(ctx, index, db, metabaseDB, repairQueue, config)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if config.EmailReminders.Enable {
		peer.Mail.EmailReminders.TestSetLinkAddress("http://" + consoleAPIAddr + "/")
	}

	return createNewSystem(prefix, log, config, peer, api, consoleAPI, ui, repairerPeer, auditorPeer, adminPeer, gcBFPeer, rangedLoopPeer), nil
}

// createNewSystem makes a new Satellite System and exposes the same interface from
// before we split out the API. In the short term this will help keep all the tests passing
// without much modification needed. However long term, we probably want to rework this
// so it represents how the satellite will run when it is made up of many processes.
func createNewSystem(name string, log *zap.Logger, config satellite.Config, peer *satellite.Core, api *satellite.API, consoleAPI *satellite.ConsoleAPI, ui *satellite.UI, repairerPeer *satellite.Repairer, auditorPeer *satellite.Auditor, adminPeer *satellite.Admin, gcBFPeer *satellite.GarbageCollectionBF, rangedLoopPeer *satellite.RangedLoop) *Satellite {
	system := &Satellite{
		Name:       name,
		Config:     config,
		Core:       peer,
		API:        api,
		ConsoleAPI: consoleAPI,
		UI:         ui,
		Repairer:   repairerPeer,
		Auditor:    auditorPeer,
		Admin:      adminPeer,
		GCBF:       gcBFPeer,
		RangedLoop: rangedLoopPeer,
	}
	system.Log = log
	system.Identity = peer.Identity
	system.DB = api.DB

	system.Dialer = api.Dialer

	system.Contact.Service = api.Contact.Service
	system.Contact.Endpoint = api.Contact.Endpoint

	system.Overlay.DB = api.Overlay.DB
	system.Overlay.Service = api.Overlay.Service
	system.Overlay.OfflineNodeEmails = peer.Overlay.OfflineNodeEmails
	system.Overlay.DQStrayNodes = peer.Overlay.DQStrayNodes

	system.NodeEvents.DB = peer.NodeEvents.DB
	system.NodeEvents.Notifier = peer.NodeEvents.Notifier
	system.NodeEvents.Chore = peer.NodeEvents.Chore

	system.Reputation.Service = peer.Reputation.Service

	system.Metainfo.Endpoint = api.Metainfo.Endpoint

	system.Userinfo.Endpoint = api.Userinfo.Endpoint

	system.Metabase.DB = api.Metainfo.Metabase

	system.Orders.DB = api.Orders.DB
	system.Orders.Endpoint = api.Orders.Endpoint
	system.Orders.Service = api.Orders.Service
	system.Orders.Chore = api.Orders.Chore

	system.Repair.Queue = repairerPeer.Queue
	system.Repair.Repairer = repairerPeer.Repairer

	system.Audit.VerifyQueue = auditorPeer.Audit.VerifyQueue
	system.Audit.ReverifyQueue = auditorPeer.Audit.ReverifyQueue
	system.Audit.Worker = auditorPeer.Audit.Worker
	system.Audit.ReverifyWorker = auditorPeer.Audit.ReverifyWorker
	system.Audit.Verifier = auditorPeer.Audit.Verifier
	system.Audit.Reverifier = auditorPeer.Audit.Reverifier
	system.Audit.Reporter = auditorPeer.Audit.Reporter
	system.Audit.ContainmentSyncChore = peer.Audit.ContainmentSyncChore

	system.GarbageCollection.Sender = peer.GarbageCollection.Sender

	system.ExpiredDeletion.Chore = peer.ExpiredDeletion.Chore
	system.ZombieDeletion.Chore = peer.ZombieDeletion.Chore

	system.Accounting.Tally = peer.Accounting.Tally
	system.Accounting.Rollup = peer.Accounting.Rollup
	system.Accounting.ProjectUsage = api.Accounting.ProjectUsage
	system.Accounting.ProjectBWCleanup = peer.Accounting.ProjectBWCleanupChore
	system.Accounting.RollupArchive = peer.Accounting.RollupArchiveChore

	system.LiveAccounting = peer.LiveAccounting

	system.GracefulExit.Endpoint = api.GracefulExit.Endpoint

	if system.Config.DisableConsoleFromSatelliteAPI {
		system.API.Console = consoleAPI.Console
		system.API.Mail = consoleAPI.Mail
		system.API.OIDC = consoleAPI.OIDC
		system.API.Analytics = consoleAPI.Analytics
		system.API.ABTesting = consoleAPI.ABTesting
		system.API.KeyManagement = consoleAPI.KeyManagement
		system.API.Payments = consoleAPI.Payments
		system.API.HealthCheck = consoleAPI.HealthCheck
		system.API.Userinfo = consoleAPI.Userinfo
		system.API.Accounting = consoleAPI.Accounting
	}

	return system
}

func (planet *Planet) newAPI(ctx context.Context, index int, identity *identity.FullIdentity, db satellite.DB, metabaseDB *metabase.DB, config satellite.Config, versionInfo version.Info) (_ *satellite.API, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-api" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	liveAccounting, err := live.OpenCache(ctx, log.Named("live-accounting"), config.LiveAccounting)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, liveAccounting)

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), config.Orders.FlushBatchSize)
	planet.databases = append(planet.databases, rollupsWriteCacheCloser{rollupsWriteCache})

	return satellite.NewAPI(log, identity, db, metabaseDB, revocationDB, liveAccounting, rollupsWriteCache, &config, versionInfo, nil)
}

func (planet *Planet) newConsoleAPI(ctx context.Context, index int, identity *identity.FullIdentity, db satellite.DB, metabaseDB *metabase.DB, config satellite.Config, versionInfo version.Info) (_ *satellite.ConsoleAPI, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-console-api" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	liveAccounting, err := live.OpenCache(ctx, log.Named("live-accounting"), config.LiveAccounting)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, liveAccounting)

	rollupsWriteCache := orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), config.Orders.FlushBatchSize)
	planet.databases = append(planet.databases, rollupsWriteCacheCloser{rollupsWriteCache})

	return satellite.NewConsoleAPI(log, identity, db, metabaseDB, revocationDB, liveAccounting, rollupsWriteCache, &config, versionInfo, nil)
}

func (planet *Planet) newUI(ctx context.Context, index int, identity *identity.FullIdentity, config satellite.Config, satelliteAddr, consoleAPIAddr string) (_ *satellite.UI, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-ui" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	return satellite.NewUI(log, identity, &config, nil, satelliteAddr, consoleAPIAddr)
}

func (planet *Planet) newAdmin(ctx context.Context, index int, identity *identity.FullIdentity, db satellite.DB, metabaseDB *metabase.DB, config satellite.Config, versionInfo version.Info) (_ *satellite.Admin, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-admin" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	liveAccounting, err := live.OpenCache(ctx, log.Named("live-accounting"), config.LiveAccounting)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, liveAccounting)

	return satellite.NewAdmin(log, identity, db, metabaseDB, liveAccounting, versionInfo, &config, nil)
}

func (planet *Planet) newRepairer(ctx context.Context, index int, identity *identity.FullIdentity, db satellite.DB, metabaseDB *metabase.DB, repairQueue queue.RepairQueue, config satellite.Config, versionInfo version.Info) (_ *satellite.Repairer, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-repairer" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	return satellite.NewRepairer(log, identity, metabaseDB, revocationDB, repairQueue, db.Buckets(), db.OverlayCache(), db.NodeEvents(), db.Reputation(), db.Containment(), versionInfo, &config, nil)
}

func (planet *Planet) newAuditor(ctx context.Context, index int, identity *identity.FullIdentity, db satellite.DB, metabaseDB *metabase.DB, config satellite.Config, versionInfo version.Info) (_ *satellite.Auditor, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-auditor" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)

	return satellite.NewAuditor(log, identity, metabaseDB, revocationDB, db.VerifyQueue(), db.ReverifyQueue(), db.OverlayCache(), db.NodeEvents(), db.Reputation(), db.Containment(), versionInfo, &config, nil)
}

type rollupsWriteCacheCloser struct {
	*orders.RollupsWriteCache
}

func (cache rollupsWriteCacheCloser) Close() error {
	return cache.RollupsWriteCache.CloseAndFlush(context.TODO())
}

func (planet *Planet) newGarbageCollectionBF(ctx context.Context, index int, db satellite.DB, metabaseDB *metabase.DB, config satellite.Config, versionInfo version.Info) (_ *satellite.GarbageCollectionBF, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-gc-bf" + strconv.Itoa(index)
	log := planet.log.Named(prefix)

	revocationDB, err := revocation.OpenDBFromCfg(ctx, config.Server.Config)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.databases = append(planet.databases, revocationDB)
	return satellite.NewGarbageCollectionBF(log, db, metabaseDB, revocationDB, versionInfo, &config, nil)
}

func (planet *Planet) newRangedLoop(ctx context.Context, index int, db satellite.DB, metabaseDB *metabase.DB, repairQueue queue.RepairQueue, config satellite.Config) (_ *satellite.RangedLoop, err error) {
	defer mon.Task()(&ctx)(&err)

	prefix := "satellite-ranged-loop" + strconv.Itoa(index)
	log := planet.log.Named(prefix)
	return satellite.NewRangedLoop(log, db, metabaseDB, repairQueue, &config, nil)
}

// atLeastOne returns 1 if value < 1, or value otherwise.
func atLeastOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
