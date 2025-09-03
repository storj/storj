// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub/v2"
	"go.uber.org/zap"
)

// EventPublisher defines an interface for publishing events.
type EventPublisher interface {
	Publish(ctx context.Context, event Event) error
}

// PubSubConfig holds configuration for Pub/Sub publisher.
// TODO: later we need to use per-bucket destinations.
type PubSubConfig struct {
	ProjectID string `help:"GCP project ID for Pub/Sub" required:"true"`
	TopicID   string `help:"GCP Pub/Sub topic to publish to" required:"true"`
}

// PubSubPublisher implements EventPublisher using Google Cloud Pub/Sub.
type PubSubPublisher struct {
	client    *pubsub.Client
	publisher *pubsub.Publisher
}

// NewPubSubPublisher initializes a Pub/Sub client and publisher.
func NewPubSubPublisher(ctx context.Context, cfg PubSubConfig) (*PubSubPublisher, error) {
	client, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}
	return &PubSubPublisher{
		client:    client,
		publisher: client.Publisher(cfg.TopicID),
	}, nil
}

// Publish sends the event to the configured Pub/Sub topic.
func (p *PubSubPublisher) Publish(ctx context.Context, event Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	res := p.publisher.Publish(ctx, &pubsub.Message{Data: b})
	_, err = res.Get(ctx)
	return err
}

// Close releases underlying Pub/Sub resources.
func (p *PubSubPublisher) Close() error {
	if p == nil {
		return nil
	}
	if p.publisher != nil {
		p.publisher.Stop()
	}
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

var _ EventPublisher = &PubSubPublisher{}

// LogPublisher implements EventPublisher by logging events.
type LogPublisher struct {
	log *zap.Logger
}

// NewLogPublisher creates a new LogPublisher.
func NewLogPublisher(log *zap.Logger) *LogPublisher {
	return &LogPublisher{
		log: log,
	}
}

// Publish logs the event.
func (l *LogPublisher) Publish(ctx context.Context, event Event) error {
	l.log.Info("Publishing event", zap.Any("event", event))
	return nil
}

var _ EventPublisher = &LogPublisher{}
