// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module provides the changestream module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	config.RegisterConfig[Config](ball, "change-stream")
	mud.Provide[*CachedPublicProjectIDs](ball, NewCachedPublicProjectIDs)
	mud.RegisterInterfaceImplementation[PublicProjectIDer, *CachedPublicProjectIDs](ball)

	config.RegisterConfig[PubSubConfig](ball, "change-stream.pubsub")
	mud.Provide[*PubSubPublisher](ball, NewPubSubPublisher)
	mud.Provide[*LogPublisher](ball, NewLogPublisher)
	mud.RegisterInterfaceImplementation[EventPublisher, *PubSubPublisher](ball)

	config.RegisterConfig[PubSubClientConfig](ball, "")
	mud.Provide[*PubSubClient](ball, NewPubSubClient)
}
