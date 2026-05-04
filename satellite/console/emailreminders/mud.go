// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package emailreminders

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Chore](ball, func(log *zap.Logger, tokens *consoleauth.Service, usersDB console.Users, mailservice *mailservice.Service, config Config, ccfg consoleweb.Config) *Chore {
		return NewChore(log, tokens, usersDB, mailservice, config, ccfg.ExternalAddress, ccfg.GeneralRequestURL, ccfg.ScheduleMeetingURL)
	})
	config.RegisterConfig[Config](ball, "email-reminders")
}
