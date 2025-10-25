// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package lifecycle allows controlling group of items.
package lifecycle

import (
	"context"
	"errors"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
)

var mon = monkit.Package()

// Group implements a collection of items that have a
// concurrent start and are closed in reverse order.
type Group struct {
	log   *zap.Logger
	items []Item

	shutdownStack sync.Once
}

// Item is the lifecycle item that group runs and closes.
type Item struct {
	Name  string
	Run   func(ctx context.Context) error
	Close func() error
}

// NewGroup creates a new group.
func NewGroup(log *zap.Logger) *Group {
	return &Group{log: log}
}

// Add adds item to the group.
func (group *Group) Add(item Item) {
	group.items = append(group.items, item)
}

// Run starts all items concurrently under group g.
func (group *Group) Run(ctx context.Context, g *errgroup.Group) {
	defer mon.Task()(&ctx)(nil)

	var started []string
	for _, item := range group.items {
		item := item
		started = append(started, item.Name)
		if item.Run == nil {
			continue
		}

		shutdownCtx, shutdownFinished := context.WithCancel(context.Background())
		go pprof.Do(ctx, pprof.Labels("name", "slow_shutdown:"+item.Name), func(ctx context.Context) {
			select {
			case <-ctx.Done():
			case <-shutdownCtx.Done():
				return
			}

			shutdownDeadline := time.NewTimer(15 * time.Second)
			defer shutdownDeadline.Stop()
			select {
			case <-shutdownDeadline.C:
				mon.Event("slow_shutdown")
				group.log.Warn("service takes long to shutdown", zap.String("name", item.Name))
				group.logStackTrace()
			case <-shutdownCtx.Done():
			}
		})

		g.Go(func() error {
			defer shutdownFinished()

			var err error
			pprof.Do(ctx, pprof.Labels("name", item.Name), func(ctx context.Context) {
				err = item.Run(ctx)
			})
			if errors.Is(ctx.Err(), context.Canceled) {
				err = errs2.IgnoreCanceled(err)
			}
			if err != nil {
				mon.Event("unexpected_shutdown")
				group.log.Error("unexpected shutdown of a runner", zap.String("name", item.Name), zap.Error(err))
			}
			return err
		})
	}

	group.log.Debug("started", zap.Strings("items", started))
}

func (group *Group) logStackTrace() {
	group.shutdownStack.Do(func() {
		buf := make([]byte, 1024*1024)
		for {
			n := runtime.Stack(buf, true)
			if n < len(buf) {
				buf = buf[:n]
				break
			}
			buf = make([]byte, 2*len(buf))
		}
		group.log.Info("slow shutdown", zap.String("stack", string(condenseStack(buf))))
	})
}

// Close closes all items in reverse order.
func (group *Group) Close() error {
	var errlist errs.Group

	for i := len(group.items) - 1; i >= 0; i-- {
		item := group.items[i]
		if item.Close == nil {
			continue
		}
		errlist.Add(item.Close())
	}

	return errlist.Err()
}
