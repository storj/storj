// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"time"

	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/avrometabase"
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
	mud.Provide[*Service](ball, NewService)
	mud.Provide[*LiveCountObserver](ball, func(db *metabase.DB, cfg Config) *LiveCountObserver {
		return NewLiveCountObserver(db, cfg.SuspiciousProcessedRatio, cfg.AsOfSystemInterval)
	})
	mud.Provide[*SegmentsCountValidation](ball, func(log *zap.Logger, db *metabase.DB, cfg Config) *SegmentsCountValidation {
		return NewSegmentsCountValidation(log, db, time.Now().Add(-cfg.SpannerStaleInterval))
	})
	mud.Provide[*RunOnce](ball, NewRunOnce)
	config.RegisterConfig[Config](ball, "ranged-loop")
	mud.RegisterImplementation[[]Observer](ball)

	mud.Implementation[[]Observer, *LiveCountObserver](ball)
	mud.Implementation[[]Observer, *SegmentsCountValidation](ball)
	mud.Tag[*SegmentsCountValidation, mud.Optional](ball, mud.Optional{})

}
