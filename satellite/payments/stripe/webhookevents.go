// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"

	wh "github.com/stripe/stripe-go/v81/webhook"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments"
)

type webhookEvents struct {
	service *Service
}

// ParseEvent parses a stripe webhookEvents event.
func (we *webhookEvents) ParseEvent(ctx context.Context, signature string, payload []byte) (*payments.WebhookEvent, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if we.service.stripeConfig.StripeWebhookSecret == "" {
		we.service.log.Debug("webhookEvents secret not set")
		return nil, nil
	}

	stripeEvent, err := wh.ConstructEvent(payload, signature, we.service.stripeConfig.StripeWebhookSecret)
	if err != nil {
		return nil, errs.New("error verifying webhookEvents event signature: %v", err)
	}

	genericEvent := &payments.WebhookEvent{
		ID:   stripeEvent.ID,
		Type: payments.WebhookEventType(stripeEvent.Type),
		Data: stripeEvent.Data.Object,
	}

	return genericEvent, nil
}
