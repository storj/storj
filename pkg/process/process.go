// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"log"

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

	Error = errs.Class("ProcessError")
)

// ID is the type used to specify a ID key in the process context
type ID string

// Service defines the interface contract for all Storj services
type Service interface {
	// Process should run the program
	Process(context.Context) error

	SetLogger(*zap.Logger) error
	SetMetricHandler(*monkit.Registry) error

	// InstanceId should return a server or process instance identifier that is
	// stable across restarts, or the empty string to use the first non-nil
	// MAC address
	InstanceId() string
}

var (
	id ID = "SrvID"
)

// Main initializes a new Service
func Main(s Service) error {
	flagfile.Load()
	ctx := context.Background()
	instanceId := s.InstanceId()
	if instanceId == "" {
		instanceId = telemetry.DefaultInstanceId()
	}

	logger, err := utils.NewLogger(*logDisposition,
		zap.Fields(zap.String(string(id), instanceId)))
	if err != nil {
		return err
	}
	defer logger.Sync()
	defer zap.ReplaceGlobals(logger)()
	defer zap.RedirectStdLog(logger)()

	ctx, cf := context.WithCancel(context.WithValue(ctx, id, instanceId))
	defer cf()

	s.SetLogger(logger)

	registry := monkit.Default
	err = initMetrics(ctx, registry, instanceId)
	if err != nil {
		logger.Error("failed to configure telemetry", zap.Error(err))
	}
	s.SetMetricHandler(registry)

	return s.Process(ctx)
}

// Must can be used for default Main error handling
func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
