// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/stripe/stripe-go/v81"
)

// WebhookEvents exposes all needed functionality to handle a stripe webhook event.
//
// architecture: Service
type WebhookEvents interface {
	// ParseEvent parses a stripe webhook event.
	ParseEvent(ctx context.Context, signature string, payload []byte) (*WebhookEvent, error)
}

// WebhookEventType represents a supported stripe webhook event type.
type WebhookEventType string

var (
	// EventTypePaymentIntentSucceeded represents the payment intent succeeded event type.
	EventTypePaymentIntentSucceeded = WebhookEventType(stripe.EventTypePaymentIntentSucceeded)

	// EventTypePaymentIntentPaymentFailed represents the payment intent payment failed event type.
	EventTypePaymentIntentPaymentFailed = WebhookEventType(stripe.EventTypePaymentIntentPaymentFailed)
)

// WebhookEvent represents a generic webhook event.
type WebhookEvent struct {
	ID   string
	Type WebhookEventType
	Data map[string]interface{}
}
