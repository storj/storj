// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("overlay error")
)

// Config is a configuration struct for everything you need to start the
// Overlay cache responsibility.
type Config struct {
	RefreshInterval time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
	Node            NodeSelectionConfig
}

// LookupConfig is a configuration struct for querying the overlay cache with one or more node IDs
type LookupConfig struct {
	NodeIDsString string `help:"one or more string-encoded node IDs, delimited by Delimiter"`
	Delimiter     string `help:"delimiter used for parsing node IDs" default:","`
}

// NodeSelectionConfig is a configuration struct to determine the minimum
// values for nodes to select
type NodeSelectionConfig struct {
	UptimeRatio       float64 `help:"a node's ratio of being up/online vs. down/offline" default:"0"`
	UptimeCount       int64   `help:"the number of times a node's uptime has been checked" default:"0"`
	AuditSuccessRatio float64 `help:"a node's ratio of successful audits" default:"0"`
	AuditCount        int64   `help:"the number of times a node has been audited" default:"0"`

	NewNodeAuditThreshold int64   `help:"the number of audits a node must have to not be considered a New Node" default:"0"`
	NewNodePercentage     float64 `help:"the percentage of new nodes allowed per request" default:"0.05"`
}

// CtxKey used for assigning cache and server
type CtxKey int

const (
	ctxKeyOverlay CtxKey = iota
	ctxKeyOverlayServer
)

// Run implements the provider.Responsibility interface. Run assumes a
// Kademlia responsibility has been started before this one.
func (c Config) Run(ctx context.Context, server *provider.Provider) (
	err error) {
	defer mon.Task()(&ctx)(&err)

	sdb, ok := ctx.Value("masterdb").(interface {
		StatDB() statdb.DB
		OverlayCache() storage.KeyValueStore
	})
	if !ok {
		return Error.Wrap(errs.New("unable to get master db instance"))
	}

	cache := NewCache(sdb.OverlayCache(), sdb.StatDB())

	ns := &pb.NodeStats{
		UptimeCount:       c.Node.UptimeCount,
		UptimeRatio:       c.Node.UptimeRatio,
		AuditSuccessRatio: c.Node.AuditSuccessRatio,
		AuditCount:        c.Node.AuditCount,
	}

	srv := NewServer(zap.L(), cache, kad, ns, c.Node.NewNodeAuditThreshold, c.Node.NewNodePercentage)
	pb.RegisterOverlayServer(server.GRPC(), srv)

	ctx2 := context.WithValue(ctx, ctxKeyOverlay, cache)
	ctx2 = context.WithValue(ctx2, ctxKeyOverlayServer, srv)
	return server.Run(ctx2)
}

// LoadFromContext gives access to the cache from the context, or returns nil
func LoadFromContext(ctx context.Context) *Cache {
	if v, ok := ctx.Value(ctxKeyOverlay).(*Cache); ok {
		return v
	}
	return nil
}

// LoadServerFromContext gives access to the overlay server from the context, or returns nil
func LoadServerFromContext(ctx context.Context) *Server {
	if v, ok := ctx.Value(ctxKeyOverlayServer).(*Server); ok {
		return v
	}
	return nil
}

// ParseIDs converts the base58check encoded node ID strings from the config into node IDs
func (c LookupConfig) ParseIDs() (ids storj.NodeIDList, err error) {
	var idErrs []error
	idStrs := strings.Split(c.NodeIDsString, c.Delimiter)
	for _, s := range idStrs {
		id, err := storj.NodeIDFromString(s)
		if err != nil {
			idErrs = append(idErrs, err)
			continue
		}
		ids = append(ids, id)
	}
	if err := utils.CombineErrors(idErrs...); err != nil {
		return nil, err
	}
	return ids, nil
}
