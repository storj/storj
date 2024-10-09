// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/process/eventkitbq"
	"storj.io/storj/satellite"
)

func cmdUIRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	satAddr := runCfg.Config.Contact.ExternalAddress
	if satAddr == "" {
		return errs.New("cannot run satellite ui if contact.external-address is not set")
	}
	apiAddress := runCfg.Config.Console.ExternalAddress
	if apiAddress == "" {
		apiAddress = runCfg.Config.Console.Address
	}
	peer, err := satellite.NewUI(log, identity, &runCfg.Config, process.AtomicLevel(cmd), satAddr, apiAddress)
	if err != nil {
		return err
	}

	if err := process.InitMetrics(ctx, log, monkit.Default, process.MetricsIDFromHostname(log), eventkitbq.BQDestination); err != nil {
		log.Warn("Failed to initialize telemetry batcher on satellite api", zap.Error(err))
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
