// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"

	"github.com/spacemonkeygo/flagfile"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/telemetry"
	"storj.io/storj/pkg/utils"
)

var (
	logDisposition = flag.String("log.disp", "prod",
		"switch to 'dev' to get more output")

	wg sync.WaitGroup

	// Error is a process error class
	Error = errs.Class("ProcessError")
	// ErrUsage is used when a user didn't use compatible or required options
	ErrUsage = errs.Class("UsageError")
)

// ID is the type used to specify a ID key in the process context
type ID string

// Service defines the interface contract for all Storj services
type Service interface {
	// Process should run the program
	Process(context.Context) error

	SetLogger(*zap.Logger) error
	SetMetricHandler(*monkit.Registry) error

	// InstanceID should return a server or process instance identifier that is
	// stable across restarts, or the empty string to use the first non-nil
	// MAC address
	InstanceID() string
}

const (
	id ID = "SrvID"
)

// Main loops over a variable number of Service args and runs StartService
// concurrently on each one. If there is an error, it will cancel the entire
// group and return the error.
func Main(s ...Service) (err error) {
	flagfile.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, service := range s {
		fmt.Printf("starting service %+v\n", service)
		srv := service
		wg.Add(1)
		go func(service Service) error {
			defer wg.Done()
			err := StartService(ctx, service)
			select {
			case <-ctx.Done():
				return nil
			}
			if err != nil {
				cancel()
				return err
			}
			return nil
		}(srv)
	}

	wg.Wait()
	return
}

// StartService will start the specified service up, load its flags,
// and set environment configs for that service
func StartService(ctx context.Context, s Service) (err error) {
	instanceID := s.InstanceID()
	if instanceID == "" {
		instanceID = telemetry.DefaultInstanceID()
	}

	registry := monkit.Default
	scope := registry.ScopeNamed("process")
	defer scope.TaskNamed("main")(&ctx)(&err)

	logger, err := utils.NewLogger(*logDisposition,
		zap.Fields(zap.String(string(id), instanceID)))
	if err != nil {
		return err
	}
	defer logger.Sync()
	defer zap.ReplaceGlobals(logger)()
	defer zap.RedirectStdLog(logger)()

	s.SetLogger(logger)
	s.SetMetricHandler(registry)

	err = initMetrics(ctx, registry, instanceID)
	if err != nil {
		logger.Error("failed to configure telemetry", zap.Error(err))
	}

	err = initDebug(ctx, logger, registry)
	if err != nil {
		logger.Error("failed to start debug endpoints", zap.Error(err))
	}

	return s.Process(ctx)
}

// Must can be used for default Main error handling
func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
