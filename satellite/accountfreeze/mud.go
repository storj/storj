// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"time"

	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "account-freeze")

	mud.Provide[ConsoleConfig](ball, func(c consoleweb.Config, pc paymentsconfig.Config) ConsoleConfig {
		cc := ConsoleConfig{
			ExternalAddress:         c.ExternalAddress,
			GeneralRequestURL:       c.GeneralRequestURL,
			FlagBots:                c.Config.Captcha.FlagBotsEnabled,
			LegacyPricingUserAgents: pc.LegacyPricingUserAgents,
		}
		if c.SingleWhiteLabel.TenantID != "" {
			cc.TenantID = &c.SingleWhiteLabel.TenantID
		}
		if c.Config.NewPricingEffectiveDate != "" {
			if t, err := time.Parse(time.RFC3339, c.Config.NewPricingEffectiveDate); err == nil {
				cc.NewPricingEffectiveDate = t
			}
		}
		return cc
	})

	mud.Provide[*Chore](ball, NewChore)
	mud.Provide[*BotFreezeChore](ball, NewBotFreezeChore)
	mud.Provide[*TrialFreezeChore](ball, NewTrialFreezeChore)
	mud.Provide[*OptOutFreezeChore](ball, NewOptOutFreezeChore)
	mud.Provide[*InactivityFreezeChore](ball, NewInactivityFreezeChore)
}
