// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package emailreminders

import (
	"context"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/mailservice"
)

var mon = monkit.Package()

// Config contains configurations for email reminders.
type Config struct {
	FirstVerificationReminder      time.Duration `help:"amount of time before sending first reminder to users who need to verify their email" default:"24h"`
	SecondVerificationReminder     time.Duration `help:"amount of time before sending second reminder to users who need to verify their email" default:"120h"`
	TrialExpirationReminder        time.Duration `help:"amount of time before trial expiration to send trial expiration reminder" default:"72h"`
	ChoreInterval                  time.Duration `help:"how often to send reminders to users who need to verify their email" default:"24h"`
	EnableTrialExpirationReminders bool          `help:"enable sending emails about trial expirations" default:"false"`
	Enable                         bool          `help:"enable sending emails reminding users to verify their email" default:"true"`
}

// Chore checks whether any emails need to be re-sent.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	tokens             *consoleauth.Service
	usersDB            console.Users
	mailService        *mailservice.Service
	config             Config
	address            string
	supportURL         string
	scheduleMeetingURL string
	useBlockingSend    bool
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, tokens *consoleauth.Service, usersDB console.Users, mailservice *mailservice.Service, config Config, address, supportURL, scheduleMeetingURL string) *Chore {
	if !strings.HasSuffix(address, "/") {
		address += "/"
	}
	return &Chore{
		log:                log,
		Loop:               sync2.NewCycle(config.ChoreInterval),
		tokens:             tokens,
		usersDB:            usersDB,
		config:             config,
		mailService:        mailservice,
		address:            address,
		supportURL:         supportURL,
		scheduleMeetingURL: scheduleMeetingURL,
		useBlockingSend:    false,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		err = chore.sendVerificationReminders(ctx)
		if err != nil {
			chore.log.Error("sending email verification reminders", zap.Error(err))
		}
		if chore.config.EnableTrialExpirationReminders {
			err = chore.sendExpirationNotifications(ctx)
			if err != nil {
				chore.log.Error("sending trial expiration notices", zap.Error(err))
			}
		}
		return nil
	})
}

func (chore *Chore) sendVerificationReminders(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()

	// cutoff to avoid emailing users multiple times due to email duplicates in the DB.
	// TODO: remove cutoff once duplicates are removed.
	cutoff := now.Add(30 * (-24 * time.Hour))

	users, err := chore.usersDB.GetUnverifiedNeedingReminder(ctx, now.Add(-chore.config.FirstVerificationReminder), now.Add(-chore.config.SecondVerificationReminder), cutoff)
	if err != nil {
		return errs.New("error getting users in need of verification reminder: %w", err)
	}
	mon.IntVal("unverified_needing_reminder").Observe(int64(len(users)))

	for _, u := range users {
		token, err := chore.tokens.CreateToken(ctx, u.ID, u.Email)

		if err != nil {
			return errs.New("error generating activation token: %w", err)
		}
		authController := consoleapi.NewAuth(chore.log, nil, nil, nil, nil, nil, nil, nil, "", chore.address, "", "", "", "", false, false, nil, "", nil)

		link := authController.ActivateAccountURL + "?token=" + token

		err = chore.sendEmail(ctx, u.Email, &console.AccountActivationEmail{
			ActivationLink: link,
			Origin:         authController.ExternalAddress,
		})
		if err != nil {
			chore.log.Error("error sending verification reminder", zap.Error(err))
			continue
		}
		if err = chore.usersDB.UpdateVerificationReminders(ctx, u.ID); err != nil {
			chore.log.Error("error updating user's last email verifcation reminder", zap.Error(err))
		}
	}
	return nil
}

func (chore *Chore) sendExpirationNotifications(ctx context.Context) (err error) {
	mon.Task()(&ctx)(&err)

	now := time.Now()

	expiring := console.TrialExpirationReminder

	// get free trial users needing reminder expiration is approaching.
	users, err := chore.usersDB.GetExpiresBeforeWithStatus(ctx, console.NoTrialNotification, now.Add(chore.config.TrialExpirationReminder))
	if err != nil {
		chore.log.Error("error getting users in need of upcoming expiration warning", zap.Error(err))
		return nil
	}
	mon.IntVal("expiring_needing_reminder").Observe(int64(len(users)))

	expirationWarning := &console.TrialExpirationReminderEmail{
		SignInLink:          chore.address + "login?source=" + analytics.SourceTrialExpiringNotice,
		Origin:              chore.address,
		ContactInfoURL:      chore.supportURL,
		ScheduleMeetingLink: chore.scheduleMeetingURL,
	}

	for _, u := range users {
		if err := chore.sendEmail(ctx, u.Email, expirationWarning); err != nil {
			chore.log.Error("error sending trial expiration reminder", zap.Error(err))
			continue
		}
		if err = chore.usersDB.Update(ctx, u.ID, console.UpdateUserRequest{TrialNotifications: &expiring}); err != nil {
			chore.log.Error("error updating user's trial_notifications", zap.Error(err))
		}
	}

	expired := console.TrialExpired

	// get free trial users needing notification that trial is expired
	users, err = chore.usersDB.GetExpiresBeforeWithStatus(ctx, console.TrialExpirationReminder, now)
	if err != nil {
		chore.log.Error("error getting users in need of expiration notice", zap.Error(err))
		return nil
	}
	mon.IntVal("expired_needing_notice").Observe(int64(len(users)))

	expirationNotice := &console.TrialExpiredEmail{
		SignInLink:          chore.address + "login?source=" + analytics.SourceTrialExpiredNotice,
		Origin:              chore.address,
		ContactInfoURL:      chore.supportURL,
		ScheduleMeetingLink: chore.scheduleMeetingURL,
	}
	for _, u := range users {
		if err := chore.sendEmail(ctx, u.Email, expirationNotice); err != nil {
			chore.log.Error("error sending trial expiration reminder", zap.Error(err))
			continue
		}

		if err = chore.usersDB.Update(ctx, u.ID, console.UpdateUserRequest{TrialNotifications: &expired}); err != nil {
			chore.log.Error("error updating user's trial_notifications", zap.Error(err))
		}
	}

	return nil
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

func (chore *Chore) sendEmail(ctx context.Context, email string, msg mailservice.Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	// blocking send allows us to verify that links are clicked in tests.
	if chore.useBlockingSend {
		err = chore.mailService.SendRendered(
			ctx,
			[]post.Address{{Address: email}},
			msg,
		)
		if err != nil {
			return err
		}
	} else {
		chore.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: email}},
			msg,
		)
	}
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
