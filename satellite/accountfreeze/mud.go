// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "account-freeze")

	mud.Provide[ConsoleConfig](ball, func(c consoleweb.Config) ConsoleConfig {
		return ConsoleConfig{
			ExternalAddress:   c.ExternalAddress,
			GeneralRequestURL: c.GeneralRequestURL,
			FlagBots:          c.Config.Captcha.FlagBotsEnabled,
		}
	})

	mud.Provide[*Chore](ball, NewChore)
	mud.Provide[*BotFreezeChore](ball, NewBotFreezeChore)
	mud.Provide[*TrialFreezeChore](ball, NewTrialFreezeChore)
}
