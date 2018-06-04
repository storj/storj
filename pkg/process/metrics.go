// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/jtolds/monkit-hw"
	"github.com/zeebo/admission/admproto"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/environment"
	"storj.io/storj/pkg/telemetry"
)

var (
	metricInterval = flag.Duration("metrics.interval", telemetry.DefaultInterval,
		"how frequently to send up telemetry")
	metricCollector = flag.String("metrics.addr", "collectora.storj.io:9000",
		"address to send telemetry to")
	metricApp = flag.String("metrics.app", filepath.Base(os.Args[0]),
		"application name for telemetry identification")
	metricAppSuffix = flag.String("metrics.app_suffix", "-dev",
		"application suffix")
)

func initMetrics(ctx context.Context, r *monkit.Registry, instanceID string) (
	err error) {
	if *metricCollector == "" || *metricInterval == 0 {
		return Error.New("telemetry disabled")
	}
	c, err := telemetry.NewClient(*metricCollector, telemetry.ClientOpts{
		Interval:      *metricInterval,
		Application:   *metricApp + *metricAppSuffix,
		Instance:      instanceID,
		Registry:      r,
		FloatEncoding: admproto.Float32Encoding,
	})
	if err != nil {
		return err
	}
	environment.Register(r)
	hw.Register(r)
	go c.Run(ctx)
	return nil
}
