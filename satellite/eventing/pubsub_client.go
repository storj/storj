// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
)

var (
	mon = monkit.Package()
)

// PubSubClientConfig holds configuration for the Pub/Sub client.
type PubSubClientConfig struct {
	ProjectID      string `help:"GCP project ID for Pub/Sub"`
	SubscriptionID string `help:"GCP Pub/Sub subscription to use"`
}

// PubSubClient is a client for receiving messages from a Pub/Sub subscription.
type PubSubClient struct {
	log     *zap.Logger
	trigger *modular.StopTrigger
	client  *pubsub.Client
	sub     *pubsub.Subscriber
}

// NewPubSubClient creates a new Pub/Sub client.
func NewPubSubClient(ctx context.Context, log *zap.Logger, cfg PubSubClientConfig, trigger *modular.StopTrigger) (_ *PubSubClient, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	sub := client.Subscriber(cfg.SubscriptionID)

	return &PubSubClient{
		log:     log,
		client:  client,
		sub:     sub,
		trigger: trigger,
	}, nil
}

// Run starts receiving messages from the Pub/Sub subscription.
func (p *PubSubClient) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer p.trigger.Cancel()
	err = p.sub.Receive(ctx, p.receive)
	return err
}

func (p *PubSubClient) receive(ctx context.Context, msg *pubsub.Message) {
	p.log.Info("Received message:", zap.String("data", string(msg.Data)))
	msg.Ack()
}

// Close releases the underlying Pub/Sub resources.
func (p *PubSubClient) Close(ctx context.Context) error {
	if p.client != nil {
		return errs.Wrap(p.client.Close())
	}
	return nil
}
