package datarepair

import(
	"context"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
	Error = errs.Class("datarepair error")
)

type Config struct {
	MaxRepair int						`help:"max repair at one time" default:"10"`
}

func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: Initialize repairer
	return server.Run(ctx)
}
