

import (
	"context"
	"time"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("discovery error")
)

// CtxKey used for assigning cache and server
type CtxKey int

const (
	ctxKeyDiscovery CtxKey = iota
	ctxKeyDiscoveryServer
)

type Config struct {
	RefreshInterval time.Duration `help:"the interval at which the cache refreshes itself in seconds" default:"1s"`
}

func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	srv := NewServer(zap.L(), cache, kad)
	pb.RegisterDiscoveryServer(server.GRPC(), srv)

	discovery := &NewDiscovery{}

	ctx := context.WithValue(ctx, ctxKeyDiscovery, discovery)
	return server.Run(ctx)
}


