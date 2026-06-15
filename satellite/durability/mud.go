// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import (
	"math/rand/v2"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is mud module definition.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "durability")
	mud.Provide[*rangedloop.SequenceObserver](ball, func(cfg Config, rcfg rangedloop.Config, db overlay.DB, metabaseDB *metabase.DB, cache *checker.ReliabilityCache) (*rangedloop.SequenceObserver, error) {

		classes, err := cfg.CreateNodeClassifiers()
		if err != nil {
			return nil, err
		}

		var reports []*Report

		for class, f := range classes {
			reports = append(reports, NewDurability(db, metabaseDB, cache, class, f, rcfg.AsOfSystemInterval))
		}

		var sequenceObservers []rangedloop.Observer
		for _, observer := range reports {
			sequenceObservers = append(sequenceObservers, observer)
		}

		// shuffle observers list to be sure that each observer will be executed first from time to time
		rand.Shuffle(len(sequenceObservers), func(i, j int) {
			sequenceObservers[i], sequenceObservers[j] = sequenceObservers[j], sequenceObservers[i]
		})
		return rangedloop.NewSequenceObserver(sequenceObservers...), nil
	})
	mud.Implementation[[]rangedloop.Observer, *rangedloop.SequenceObserver](ball)
	mud.Tag[*rangedloop.SequenceObserver, mud.Optional](ball, mud.Optional{})
}
