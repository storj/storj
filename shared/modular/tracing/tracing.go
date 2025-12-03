// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tracing

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	stracing "storj.io/common/tracing"
	jaeger "storj.io/monkit-jaeger"
)

// Config holds the configuration for distributed tracing.
type Config struct {
	Enabled      bool          `help:"whether tracing collector is enabled" default:"true"`
	SamplingRate float64       `help:"how frequent to sample traces"`
	App          string        `help:"application name for tracing identification"`
	AgentAddr    string        `help:"address for jaeger agent" default:"agent.tracing.datasci.storj.io:5775"`
	BufferSize   int           `help:"buffer size for collector batch packet size"`
	QueueSize    int           `help:"buffer size for collector queue size"`
	Interval     time.Duration `help:"how frequently to flush traces to tracing agent" default:"15s"`
	HostRegex    string        `help:"the possible hostnames that trace-host designated traces can be sent to" default:"\\.storj\\.tools:[0-9]+$"`
}

var (
	maxInstanceLength = 63
)

const (
	instanceIDKey = "instanceID"
	hostnameKey   = "hostname"
)

// Tracing manages distributed tracing for the application.
type Tracing struct {
	registry *monkit.Registry
	cfg      Config
	log      *zap.Logger

	nodeID storj.NodeID

	unregister func()
	cancel     context.CancelFunc
}

// NewTracing creates a new Tracing instance.
func NewTracing(log *zap.Logger, nodeID storj.NodeID, cfg Config) *Tracing {
	return &Tracing{
		registry: monkit.Default,
		cfg:      cfg,
		log:      log,
		nodeID:   nodeID,
	}
}

type traceCollectorFactoryFunc func(hostTarget string) (jaeger.ClosableTraceCollector, error)

func (f traceCollectorFactoryFunc) MakeCollector(hostTarget string) (jaeger.ClosableTraceCollector, error) {
	return f(hostTarget)
}

// Run starts the tracing collector if enabled.
func (t *Tracing) Run(ctx context.Context) error {
	if !t.cfg.Enabled {
		t.log.Debug("Anonymized tracing disabled")
		return nil
	}
	t.log.Info("Anonymized tracing enabled")

	if t.registry == nil {
		t.registry = monkit.Default
	}

	processName := t.cfg.App
	if processName == "" {
		processName = strings.TrimSuffix(filepath.Base(os.Args[0]), "-exe")
	}
	hostRegex, err := regexp.Compile(t.cfg.HostRegex)
	if err != nil {
		return errs.Wrap(err)
	}

	var processInfo []jaeger.Tag
	hostname, err := os.Hostname()
	if err != nil {
		t.log.Error("Could not read hostname for tracing setup", zap.Error(err))
	} else {
		processInfo = append(processInfo, jaeger.Tag{
			Key:   hostnameKey,
			Value: hostname,
		})
	}

	processInfo = append(processInfo, jaeger.Tag{
		Key:   instanceIDKey,
		Value: t.nodeID.String(),
	})

	if len(processName) > maxInstanceLength {
		processName = processName[:maxInstanceLength]
	}
	collector, err := jaeger.NewThriftCollector(t.log, t.cfg.AgentAddr, processName, processInfo, t.cfg.BufferSize, t.cfg.QueueSize, t.cfg.Interval)
	if err != nil {
		return errs.Wrap(err)
	}

	collectorCtx, collectorCtxCancel := context.WithCancel(ctx)
	t.cancel = collectorCtxCancel
	var eg errgroup.Group
	eg.Go(func() error {
		collector.Run(collectorCtx)
		return nil
	})

	t.unregister = jaeger.RegisterJaeger(t.registry, collector, jaeger.Options{
		Fraction: t.cfg.SamplingRate,
		Excluded: stracing.IsExcluded,
		CollectorFactory: traceCollectorFactoryFunc(func(targetHost string) (jaeger.ClosableTraceCollector, error) {
			targetCollector, err := jaeger.NewThriftCollector(t.log, targetHost, processName, processInfo, t.cfg.BufferSize, t.cfg.QueueSize, t.cfg.Interval)
			if err != nil {
				return nil, err
			}
			targetCollectorCtx, targetCollectorCancel := context.WithCancel(collectorCtx)
			eg.Go(func() error {
				targetCollector.Run(targetCollectorCtx)
				return nil
			})
			return &closableCollector{
				cancel:                 targetCollectorCancel,
				ClosableTraceCollector: targetCollector,
			}, nil
		}),
		CollectorFactoryHostMatch: hostRegex,
	})
	return eg.Wait()
}

// Close stops the tracing collector.
func (t *Tracing) Close() error {
	if t.unregister != nil {
		t.unregister()
		t.unregister = nil
	}
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	return nil
}

type closableCollector struct {
	jaeger.ClosableTraceCollector
	cancel func()
}

func (collector *closableCollector) Close() error {
	collector.cancel()
	return collector.ClosableTraceCollector.Close()
}
