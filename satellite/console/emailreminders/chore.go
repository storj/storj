// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package emailreminders

import (
	"context"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/mailservice"
)

var mon = monkit.Package()

// Config contains configurations for email reminders.
type Config struct {
	FirstVerificationReminder  time.Duration `help:"amount of time before sending first reminder to users who need to verify their email" default:"24h"`
	SecondVerificationReminder time.Duration `help:"amount of time before sending second reminder to users who need to verify their email" default:"120h"`
	ChoreInterval              time.Duration `help:"how often to send reminders to users who need to verify their email" default:"24h"`
	Enable                     bool          `help:"enable sending emails reminding users to verify their email" default:"true"`
}

// Chore checks whether any emails need to be re-sent.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	tokens          *consoleauth.Service
	usersDB         console.Users
	mailService     *mailservice.Service
	config          Config
	address         string
	useBlockingSend bool
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, tokens *consoleauth.Service, usersDB console.Users, mailservice *mailservice.Service, config Config, address string) *Chore {
	if !strings.HasSuffix(address, "/") {
		address += "/"
	}
	return &Chore{
		log:             log,
		Loop:            sync2.NewCycle(config.ChoreInterval),
		tokens:          tokens,
		usersDB:         usersDB,
		config:          config,
		mailService:     mailservice,
		address:         address,
		useBlockingSend: false,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		now := time.Now()

		// cutoff to avoid emailing users multiple times due to email duplicates in the DB.
		// TODO: remove cutoff once duplicates are removed.
		cutoff := now.Add(30 * (-24 * time.Hour))

		users, err := chore.usersDB.GetUnverifiedNeedingReminder(ctx, now.Add(-chore.config.FirstVerificationReminder), now.Add(-chore.config.SecondVerificationReminder), cutoff)
		if err != nil {
			chore.log.Error("error getting users in need of reminder", zap.Error(err))
			return nil
		}
		mon.IntVal("unverified_needing_reminder").Observe(int64(len(users)))

		for _, u := range users {
			token, err := chore.tokens.CreateToken(ctx, u.ID, u.Email)

			if err != nil {
				chore.log.Error("error generating activation token", zap.Error(err))
				return nil
			}
			authController := consoleapi.NewAuth(chore.log, nil, nil, nil, nil, nil, "", chore.address, "", "", "", "", false)

			link := authController.ActivateAccountURL + "?token=" + token

			// blocking send allows us to verify that links are clicked in tests.
			if chore.useBlockingSend {
				err = chore.mailService.SendRendered(
					ctx,
					[]post.Address{{Address: u.Email}},
					&console.AccountActivationEmail{
						ActivationLink: link,
						Origin:         authController.ExternalAddress,
					},
				)
				if err != nil {
					chore.log.Error("error sending email reminder", zap.Error(err))
					continue
				}
			} else {
				chore.mailService.SendRenderedAsync(
					ctx,
					[]post.Address{{Address: u.Email}},
					&console.AccountActivationEmail{
						ActivationLink: link,
						Origin:         authController.ExternalAddress,
					},
				)
			}
			if err = chore.usersDB.UpdateVerificationReminders(ctx, u.ID); err != nil {
				chore.log.Error("error updating user's last email verifcation reminder", zap.Error(err))
			}
		}
		return nil
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// TestSetLinkAddress allows the email link address to be reconfigured.
// The address points to the satellite web server's external address.
// In the test environment the external address is not set by a config.
// It is an internal address, and we don't know what the port is until after it
// has been assigned. With this method, we get the address from the api in testplanet
// and assign it here.
func (chore *Chore) TestSetLinkAddress(address string) {
	chore.address = address
}

// TestUseBlockingSend allows us to set the chore to use a blocking send method.
// Using a blocking send method allows us to test that links are clicked without
// potential race conditions.
func (chore *Chore) TestUseBlockingSend() {
	chore.useBlockingSend = true
}
