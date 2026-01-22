// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/storj/private/emptyfs"
	"storj.io/storj/satellite/admin/auditlogger"
	legacyAdmin "storj.io/storj/satellite/admin/legacy"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
)

// Assets contains either the built admin/ui or it is nil.
var Assets fs.FS = emptyfs.FS{}

var (
	// Error is the error class that wraps all the errors returned by this package.
	Error = errs.Class("satellite-admin")

	mon = monkit.Package()
)

// Config defines configuration for the satellite administration server.
type Config struct {
	Address         string `help:"admin peer http listening address" releaseDefault:"" devDefault:""`
	ExternalAddress string `help:"external endpoint of the satellite admin" default:""`

	StaticDir string `help:"an alternate directory path which contains the static assets for the satellite administration web app. When empty, it uses the embedded assets"`

	BypassAuth bool `help:"ignore authentication for local development" default:"false" hidden:"true"`
	// hidden for now because it is provided by the legacy admin server.
	AllowedOauthHost                   string `help:"the oauth host allowed to host the backoffice."`
	PendingDeleteUserCleanupEnabled    bool   `help:"whether the pending delete data deletion chore is enabled for users." default:"false" hidden:"true"`
	PendingDeleteProjectCleanupEnabled bool   `help:"whether the pending delete data deletion chore is enabled for projects." default:"false" hidden:"true"`

	UserGroupsRoleAdmin           []string `help:"the list of groups whose users has the administration role"   releaseDefault:"" devDefault:""`
	UserGroupsRoleViewer          []string `help:"the list of groups whose users has the viewer role"           releaseDefault:"" devDefault:""`
	UserGroupsRoleCustomerSupport []string `help:"the list of groups whose users has the customer support role" releaseDefault:"" devDefault:""`
	UserGroupsRoleFinanceManager  []string `help:"the list of groups whose users has the finance manager role"  releaseDefault:"" devDefault:""`

	AuditLogger auditlogger.Config

	Legacy legacyAdmin.Config
}

// Server serves the API endpoints and the web application to allow preforming satellite
// administration tasks.
type Server struct {
	log      *zap.Logger
	listener net.Listener

	config Config

	server       http.Server
	legacyServer *legacyAdmin.Server
}

// NewServer creates a satellite administration server instance with the provided dependencies and
// configurations.
//
// When listener is nil, Server.Run is a noop.
func NewServer(
	log *zap.Logger,
	listener net.Listener,
	db legacyAdmin.DB,
	metabaseDB *metabase.DB,
	buckets *buckets.Service,
	restKeys restapikeys.Service,
	freezeAccounts *console.AccountFreezeService,
	analyticsService *analytics.Service,
	accounts payments.Accounts,
	service *Service,
	entitlements *entitlements.Service,
	placement nodeselection.PlacementDefinitions,
	console consoleweb.Config,
	entitlementsCfg entitlements.Config,
	config Config,
) *Server {
	server := &Server{
		log:      log,
		listener: listener,
		config:   config,
	}

	root := mux.NewRouter()

	// API endpoints.
	// API generator already adds the PathPrefix to each route.
	NewPlacementManagement(log, mon, service, root)
	NewProductManagement(log, mon, service, root)
	NewUserManagement(log, mon, service, root, service.authorizer)
	NewProjectManagement(log, mon, service, root, service.authorizer)
	NewSettings(log, mon, service, root, service.authorizer)
	NewSearch(log, mon, service, root, service.authorizer)
	NewChangeHistory(log, mon, service, root, service.authorizer)

	server.legacyServer = legacyAdmin.NewServer(
		log.Named("legacy-admin"),
		listener,
		db,
		metabaseDB,
		buckets,
		restKeys,
		freezeAccounts,
		analyticsService,
		accounts,
		entitlements,
		placement,
		console,
		entitlementsCfg,
		config.Legacy,
		root.PathPrefix(legacyAdmin.PathPrefix).Subrouter(),
	)

	// Static assets for the web interface.
	// This handler must be the last one because it uses the root as prefix, otherwise, it will serve
	// all the paths defined by the handlers set after this one.

	var staticPath string
	var fileSystem http.FileSystem
	if config.StaticDir == "" {
		fileSystem = http.FS(Assets)
		staticPath = "/static/build"
	} else {
		fileSystem = http.Dir(config.StaticDir)
		staticPath = "/static"
	}
	staticHandler := http.StripPrefix(staticPath, http.FileServer(fileSystem))
	root.PathPrefix("/static/").Handler(staticHandler)

	root.PathPrefix("").Handler(http.HandlerFunc(server.uiHandler))

	server.server.Handler = root

	return server
}

func (server *Server) uiHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	header.Set("Content-Type", "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")

	if server.config.StaticDir == "" {
		content, err := fs.ReadFile(Assets, "index.html")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				server.log.Error("admin UI was not embedded", zap.Error(err))
			} else {
				server.log.Error("error loading index.html", zap.String("path", "index.html"), zap.Error(err))
			}

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(content)
		return
	}

	indexPath := filepath.Join(server.config.StaticDir, "build", "index.html")
	file, err := os.Open(indexPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			server.log.Error("index.html was not generated. run 'npm run build' in the "+server.config.StaticDir+" directory", zap.Error(err))
		} else {
			server.log.Error("error loading index.html", zap.String("path", indexPath), zap.Error(err))
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			server.log.Error("error closing index.html", zap.String("path", indexPath), zap.Error(err))
		}
	}()

	info, err := file.Stat()
	if err != nil {
		server.log.Error("failed to retrieve index.html file info", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, indexPath, info.ModTime(), file)
}

// Run starts the administration HTTP server using the provided listener.
// If listener is nil, it does nothing and return nil.
func (server *Server) Run(ctx context.Context) error {
	if server.listener == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(server.server.Shutdown(context.Background()))
	})
	group.Go(func() error {
		defer cancel()
		err := server.server.Serve(server.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return Error.Wrap(err)
	})
	return group.Wait()
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return Error.Wrap(server.server.Close())
}

// SetLegacyNowFn sets the function to get the current time in the legacy admin server.
func (server *Server) SetLegacyNowFn(f func() time.Time) {
	server.legacyServer.SetNow(f)
}

// SetLegacyAllowedOauthHost allows tests to set which address to recognize as belonging to the OAuth proxy
// in the legacy server.
func (server *Server) SetLegacyAllowedOauthHost(address string) {
	server.legacyServer.SetAllowedOauthHost(address)
}
