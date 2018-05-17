// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"

	"github.com/google/uuid"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/utils"
)

// ID is the type used to specify a ID key in the process context
type ID string

// Service defines the interface contract for all Storj services
type Service interface {
	Process(context.Context) error
	SetLogger(*zap.Logger) error
	SetMetricHandler(*monkit.Registry) error
}

var (
	id ID = "SrvID"
)

// Main initializes a new Service
func Main(s Service) error {
	flag.Parse()
	ctx := context.Background()
	uid := uuid.New().String()

	logger, err := utils.NewLogger("", zap.Fields(zap.String("SrvID", uid)))
	if err != nil {
		return err
	}
	defer logger.Sync()

	ctx, cf := context.WithCancel(context.WithValue(ctx, id, uid))
	defer cf()

	s.SetLogger(logger)
	s.SetMetricHandler(monkit.NewRegistry())

	return s.Process(ctx)
}
