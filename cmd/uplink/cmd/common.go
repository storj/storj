// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink/telemetry"
)

func getConfDir() string {
	if param := cfgstruct.FindConfigDirParam(); param != "" {
		return param
	}
	return fpath.ApplicationDir("storj", "uplink")
}

func withTelemetry(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	ctx, _ := process.Ctx(cmd)
	addr := cmd.Flag("metrics.addr")
	if addr == nil || addr.Value.String() == "" {
		return ctx, nil
	}

	return telemetry.Enable(ctx)
}

func enableTracing(config map[string]interface{}) {
	config["tracing.enabled"] = true
	config["tracing.sample"] = 1
}
