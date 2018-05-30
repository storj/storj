// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// ServiceFunc allows one to implement a Service in terms of simply the Process
// method
type ServiceFunc func(context.Context) error

// Process implements the Service interface and simply calls f
func (f ServiceFunc) Process(ctx context.Context) error { return f(ctx) }

// SetLogger implements the Service interface but is a no-op
func (f ServiceFunc) SetLogger(*zap.Logger) error { return nil }

// SetMetricHandler implements the Service interface but is a no-op
func (f ServiceFunc) SetMetricHandler(*monkit.Registry) error { return nil }

// InstanceID implements the Service interface and expects default behavior
func (f ServiceFunc) InstanceID() string { return "" }
