// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package profiler

import (
	"cloud.google.com/go/profiler"
	"go.uber.org/zap"

	"storj.io/common/version"
)

// Config contains configuration for Profiler.
type Config struct {
	Name    string `help:"provide the name of the peer to enable continuous cpu/mem profiling for"`
	Project string `help:"provide the google project id for continuous profiling (required only for non-k8s environments)"`
}

// Profiler is a component that initializes the Google continuous profiler.
type Profiler struct {
	cfg Config
	log *zap.Logger
}

// NewProfiler creates a new Profiler.
func NewProfiler(cfg Config, log *zap.Logger) *Profiler {
	return &Profiler{
		cfg: cfg,
		log: log,
	}
}

// Run starts the profiler.
func (p *Profiler) Run() error {
	name := p.cfg.Name
	if name != "" {
		p.log.Info("starting Google continuous profiler", zap.String("name", name), zap.String("project", p.cfg.Project))
		info := version.Build
		config := profiler.Config{
			Service:        name,
			ServiceVersion: info.Version.String(),
			ProjectID:      p.cfg.Project,
		}

		if err := profiler.Start(config); err != nil {
			p.log.Warn("Couldn't start the profiler", zap.Error(err))
		}
		p.log.Debug("success debug profiler init")
	}
	return nil
}
