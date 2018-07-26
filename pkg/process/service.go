// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/telemetry"
	"storj.io/storj/pkg/utils"
)

var (
	logDisposition = flag.String("log.disp", "dev",
		"switch to 'prod' to get less output")

	// Error is a process error class
	Error = errs.Class("proc error")

	// ErrUsage is a process error class
	ErrUsage = errs.Class("usage error")

	// ErrLogger Class
	ErrLogger = errs.Class("Logger Error")

	// ErrMetricHandler Class
	ErrMetricHandler = errs.Class("Metric Handler Error")

	//ErrProcess Class
	ErrProcess = errs.Class("Process Error")
)

type idKey string

const (
	id idKey = "SrvID"
)

// Service defines the interface contract for all Storj services
type Service interface {
	// Process should run the program
	Process(ctx context.Context, cmd *cobra.Command, args []string) error

	SetLogger(*zap.Logger) error
	SetMetricHandler(*monkit.Registry) error

	// InstanceID should return a server or process instance identifier that is
	// stable across restarts, or the empty string to use the first non-nil
	// MAC address
	InstanceID() string
}

// ServiceFunc allows one to implement a Service in terms of simply the Process
// method
type ServiceFunc func(ctx context.Context, cmd *cobra.Command,
	args []string) error

// Process implements the Service interface and simply calls f
func (f ServiceFunc) Process(ctx context.Context, cmd *cobra.Command,
	args []string) error {
	return f(ctx, cmd, args)
}

// SetLogger implements the Service interface but is a no-op
func (f ServiceFunc) SetLogger(*zap.Logger) error { return nil }

// SetMetricHandler implements the Service interface but is a no-op
func (f ServiceFunc) SetMetricHandler(*monkit.Registry) error { return nil }

// InstanceID implements the Service interface and expects default behavior
func (f ServiceFunc) InstanceID() string { return "" }

// CtxRun is useful for generating cobra.Command.RunE methods that get
// a context
func CtxRun(fn func(ctx context.Context, cmd *cobra.Command,
	args []string) error) func(cmd *cobra.Command, args []string) error {
	return CtxService(ServiceFunc(fn))
}

// CtxService turns a Service into a cobra.Command.RunE method
func CtxService(s Service) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) (err error) {
		instanceID := s.InstanceID()
		if instanceID == "" {
			instanceID = telemetry.DefaultInstanceID()
		}

		ctx := context.WithValue(context.Background(), id, instanceID)

		registry := monkit.Default
		scope := registry.ScopeNamed("process")
		defer scope.TaskNamed("main")(&ctx)(&err)

		logger, err := utils.NewLogger(*logDisposition,
			zap.Fields(zap.String(string(id), instanceID)))
		if err != nil {
			return err
		}
		defer func() { _ = logger.Sync() }()

		defer zap.ReplaceGlobals(logger)()
		defer zap.RedirectStdLog(logger)()

		if err := s.SetLogger(logger); err != nil {
			logger.Error("failed to configure logger", zap.Error(err))
		}

		if err := s.SetMetricHandler(registry); err != nil {
			logger.Error("failed to configure metric handler", zap.Error(err))
		}

		err = initMetrics(ctx, registry, instanceID)
		if err != nil {
			logger.Error("failed to configure telemetry", zap.Error(err))
		}

		err = initDebug(logger, registry)
		if err != nil {
			logger.Error("failed to start debug endpoints", zap.Error(err))
		}

		return s.Process(ctx, cmd, args)
	}
}
