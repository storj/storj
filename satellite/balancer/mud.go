// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package balancer

import (
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers the balancer components.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "balancer")
	mud.Provide[*Balancer](ball, NewBalancer)
	mud.Tag[*Balancer, mud.Optional](ball, mud.Optional{})
	mud.Implementation[[]rangedloop.Observer, *Balancer](ball)

	config.RegisterConfig[WorkerConfig](ball, "balancer.worker")
	mud.Provide[*Worker](ball, NewWorker)
	mud.Tag[*Worker, mud.Optional](ball, mud.Optional{})

}
