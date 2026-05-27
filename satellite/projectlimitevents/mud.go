// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package projectlimitevents

import (
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.View[*mailservice.Service, MailSender](ball, func(service *mailservice.Service) MailSender {
		return service
	})
	mud.Provide[*Chore](ball, NewChore)
	config.RegisterConfig[Config](ball, "project-limit-events")
}
