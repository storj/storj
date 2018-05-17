// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

type ServiceFunc func(context.Context) error

func (f ServiceFunc) Process(ctx context.Context) error       { return f(ctx) }
func (f ServiceFunc) SetLogger(*zap.Logger) error             { return nil }
func (f ServiceFunc) SetMetricHandler(*monkit.Registry) error { return nil }
func (f ServiceFunc) InstanceId() string                      { return "" }
