// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	hw "github.com/jtolds/monkit-hw/v2"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/environment"
	"github.com/spacemonkeygo/monkit/v3/present"
	"go.uber.org/zap"

	"storj.io/common/identity"
	jaeger "storj.io/monkit-jaeger"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/telemetry"
	"storj.io/storj/private/version"
)

var (
	metricInterval       = flag.Duration("metrics.interval", telemetry.DefaultInterval, "how frequently to send up telemetry")
	metricCollector      = flag.String("metrics.addr", flagDefault("", "collectora.storj.io:9000"), "address to send telemetry to")
	metricApp            = flag.String("metrics.app", filepath.Base(os.Args[0]), "application name for telemetry identification")
	metricAppSuffix      = flag.String("metrics.app-suffix", flagDefault("-dev", "-release"), "application suffix")
	metricInstancePrefix = flag.String("metrics.instance-prefix", "", "instance id prefix")
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
	if r == nil {
		r = monkit.Default
	}
	environment.Register(r)
	hw.Register(r)
	r.ScopeNamed("env").Chain(monkit.StatSourceFunc(version.Build.Stats))

	log = log.Named("telemetry")
	// if *metricCollector == "" || *metricInterval == 0 {
	// 	log.Info("disabled")
	// 	return nil
	// }

	if instanceID == "" {
		instanceID = telemetry.DefaultInstanceID()
	}
	instanceID = *metricInstancePrefix + instanceID
	if len(instanceID) > maxInstanceLength {
		instanceID = instanceID[:maxInstanceLength]
	}
	// c, err := telemetry.NewClient(log, *metricCollector, telemetry.ClientOpts{
	// 	Interval:      *metricInterval,
	// 	Application:   *metricApp + *metricAppSuffix,
	// 	Instance:      instanceID,
	// 	Registry:      r,
	// 	FloatEncoding: admproto.Float32Encoding,
	// })
	if err != nil {
		return err
	}
	go http.ListenAndServe(*metricCollector, present.HTTP(r))
	collector, err := jaeger.NewUDPCollector("localhost:5775", 600, *metricApp, []jaeger.Tag{
		jaeger.Tag{
			Key:   "instance-id",
			Value: &instanceID,
		},
	})
	if err != nil {
		panic(err)
	}
	jaeger.RegisterJaeger(r, collector, jaeger.Options{
		Fraction: 1,
	})
	// go c.Run(ctx)
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

// InitMetricsWithHostname initializes telemetry reporting, using the hostname as the telemetry instance ID.
func InitMetricsWithHostname(ctx context.Context, log *zap.Logger, r *monkit.Registry) error {
	var metricsID string
	hostname, err := os.Hostname()
	if err != nil {
		log.Sugar().Errorf("Could not read hostname for telemetry setup: %v", err)
		metricsID = "" // InitMetrics() will fill in a default value
	} else {
		metricsID = strings.ReplaceAll(hostname, ".", "_")
	}
	return InitMetrics(ctx, log, r, metricsID)
}
