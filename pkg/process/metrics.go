// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	hw "github.com/jtolds/monkit-hw"
	"github.com/zeebo/admission/admproto"
	"go.uber.org/zap"
	zipkin "gopkg.in/spacemonkeygo/monkit-zipkin.v2"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/environment"

	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/telemetry"
)

var (
	metricInterval       = flag.Duration("metrics.interval", telemetry.DefaultInterval, "how frequently to send up telemetry")
	metricCollector      = flag.String("metrics.addr", flagDefault("", "collectora.storj.io:9000"), "address to send telemetry to")
	metricApp            = flag.String("metrics.app", filepath.Base(os.Args[0]), "application name for telemetry identification")
	metricAppSuffix      = flag.String("metrics.app-suffix", flagDefault("-dev", "-release"), "application suffix")
	metricInstancePrefix = flag.String("metrics.instance-prefix", "", "instance id prefix")

	zipkinCollector = flag.String("zipkin.addr", "", "address to send traces to")
	zipkinFraction  = flag.Float64("zipkin.fraction", 0, "fraction of traces to observe")
	zipkinDebug     = flag.Bool("zipkin.debug", false, "whether to set debug flag on new traces")
	zipkinBuffer    = flag.Int("zipkin.buffer", 64, "how many outstanding spans can be buffered before being dropped")
)

const (
	maxInstanceLength = 52
)

func flagDefault(dev, release string) string {
	if cfgstruct.DefaultsType() == "release" {
		return release
	}
	return dev
}

// InitMetrics initializes telemetry reporting. Makes a telemetry.Client and calls
// its Run() method in a goroutine.
func InitMetrics(ctx context.Context, log *zap.Logger, r *monkit.Registry, instanceID string) (err error) {
	if *metricCollector == "" || *metricInterval == 0 {
		return Error.New("telemetry disabled")
	}
	if r == nil {
		r = monkit.Default
	}
	if instanceID == "" {
		instanceID = telemetry.DefaultInstanceID()
	}
	instanceID = *metricInstancePrefix + instanceID
	if len(instanceID) > maxInstanceLength {
		instanceID = instanceID[:maxInstanceLength]
	}
	c, err := telemetry.NewClient(log, *metricCollector, telemetry.ClientOpts{
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
	if *zipkinCollector != "" && *zipkinFraction > 0 {
		collector, err := zipkin.NewUDPCollector(*zipkinCollector, *zipkinBuffer)
		if err != nil {
			return err
		}
		zipkin.RegisterZipkin(r, collector, zipkin.Options{
			Fraction: *zipkinFraction,
			Debug:    *zipkinDebug,
		})
	}
	r.ScopeNamed("env").Chain("version", monkit.StatSourceFunc(version.Build.Stats))
	go c.Run(ctx)
	return nil
}

// InitMetricsWithCertPath initializes telemetry reporting, using the node ID
// corresponding to the given certificate as the telemetry instance ID.
func InitMetricsWithCertPath(ctx context.Context, log *zap.Logger, r *monkit.Registry, certPath string) error {
	var metricsID string
	nodeID, err := identity.NodeIDFromCertPath(certPath)
	if err != nil {
		log.Sugar().Errorf("Could not read identity for telemetry setup: %v", err)
		metricsID = "" // InitMetrics() will fill in a default value
	} else {
		metricsID = nodeID.String()
	}
	return InitMetrics(ctx, log, r, metricsID)
}
