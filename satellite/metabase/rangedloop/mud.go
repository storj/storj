// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/avrometabase"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// SplitterConfig contains configurable values for the Avro GCS segment splitter.
type SplitterConfig struct {
	Bucket           string `required:"true" help:"GCS bucket where the Avro files are stored."`
	SegmentPattern   string `default:"segments.avro-*" help:"Pattern for segment Avro files."`
	NodeAliasPattern string `default:"node_aliases.avro-*" help:"Pattern for node aliases Avro files."`
}

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*MetabaseRangeSplitter](ball, NewMetabaseRangeSplitter)
	config.RegisterConfig[SplitterConfig](ball, "avro.gcs")
	mud.Provide[*AvroSegmentsSplitter](ball, func(cfg SplitterConfig) *AvroSegmentsSplitter {
		nodeAliasesIterator := avrometabase.NewGCSIterator(cfg.Bucket, cfg.NodeAliasPattern)
		segmentIterator := avrometabase.NewGCSIterator(cfg.Bucket, cfg.SegmentPattern)
		return NewAvroSegmentsSplitter(segmentIterator, nodeAliasesIterator)
	})
	mud.RegisterInterfaceImplementation[RangeSplitter, *MetabaseRangeSplitter](ball)
	// Only the jobs that pin a safepoint themselves may be configured with one;
	// anywhere else the flag would look like it pins the snapshot that bloom
	// filter generation is validated against, without anyone holding it. Those
	// jobs build their own service, they do not take this component.
	mud.Provide[*Service](ball, func(log *zap.Logger, config Config, provider RangeSplitter, observers []Observer) (*Service, error) {
		if err := rejectSafepoint(config); err != nil {
			return nil, err
		}
		return NewService(log, config, provider, observers), nil
	})
	mud.Provide[*LiveCountObserver](ball, func(db *metabase.DB, cfg Config) *LiveCountObserver {
		return NewLiveCountObserver(db, cfg.SuspiciousProcessedRatio, cfg.AsOfSystemInterval)
	})
	mud.Provide[*RunOnce](ball, func(log *zap.Logger, stop *modular.StopTrigger, config Config, provider RangeSplitter, observers []Observer) (*RunOnce, error) {
		if err := rejectSafepoint(config); err != nil {
			return nil, err
		}
		return NewRunOnce(log, stop, config, provider, observers), nil
	})
	config.RegisterConfig[Config](ball, "ranged-loop")
	mud.RegisterImplementation[[]Observer](ball)

	mud.Implementation[[]Observer, *LiveCountObserver](ball)

}

// rejectSafepoint fails when a safepoint is configured for a run that never
// holds one.
func rejectSafepoint(config Config) error {
	if config.Safepoint.Enabled() {
		return errs.New("ranged-loop.safepoint.pd-endpoints requires the gc-bf-once subcommand")
	}
	return nil
}
