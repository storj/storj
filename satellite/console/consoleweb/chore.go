// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
)

// Chore checks whether any emails need to be re-sent.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	Loop *sync2.Cycle

	service     *console.Service
	mailsender  *mailservice.Sender
	mailService *mailservice.Service
	config      Config
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, service *console.Service, mailservice *mailservice.Service, config Config) *Chore {
	return &Chore{
		log:         log,
		Loop:        sync2.NewCycle(time.Nanosecond),
		service:     service,
		config:      config,
		mailService: mailservice,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		users, err := chore.service.GetUnverifiedNeedingReminder(ctx)
		for _, u := range users {

			token, err := chore.service.GenerateActivationToken(ctx, u.ID, u.Email)

			if err != nil {
				chore.log.Error("error generating activation token", zap.Error(err))
				return nil
			}
			authController := consoleapi.NewAuth(zap.L(), nil, nil, nil, nil, nil, "", "", "", "")

			link := authController.ActivateAccountURL + "?token=" + token
			userName := u.ShortName
			if u.ShortName == "" {
				userName = u.FullName
			}

			chore.mailService.SendRenderedAsync(
				ctx,
				[]post.Address{{Address: u.Email, Name: userName}},
				&consoleql.AccountActivationEmail{
					ActivationLink: link,
					Origin:         authController.ExternalAddress,
					UserName:       userName,
				},
			)
			if err = chore.service.UpdateEmailVerificationReminder(ctx, time.Now().UTC()); err != nil {
				chore.log.Error("error updating user's last email verifcation reminder", zap.Error(err))
			}
		}
		return err
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
