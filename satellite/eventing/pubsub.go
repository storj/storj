// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"cloud.google.com/go/pubsub/v2"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Publisher defines an interface for publishing events.
type Publisher interface {
	Publish(ctx context.Context, event any) error
	io.Closer
}

// NewPublisher creates a new EventPublisher based on the provided topic name.
func NewPublisher(ctx context.Context, topicName string) (Publisher, error) {
	if topicName == "@log" {
		return NewLogPublisher(zap.L()), nil
	}

	projectID, topicID, err := ParseTopicName(topicName)
	if err != nil {
		return nil, err
	}

	return NewPubSubPublisher(ctx, PubSubConfig{
		ProjectID: projectID,
		TopicID:   topicID,
	})
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
		return nil, errs.Wrap(err)
	}
	return &PubSubPublisher{
		client:    client,
		publisher: client.Publisher(cfg.TopicID),
	}, nil
}

// Publish sends the event to the configured Pub/Sub topic.
func (p *PubSubPublisher) Publish(ctx context.Context, event any) error {
	b, err := json.Marshal(event)
	if err != nil {
		return errs.Wrap(err)
	}
	res := p.publisher.Publish(ctx, &pubsub.Message{Data: b})
	_, err = res.Get(ctx)
	return errs.Wrap(err)
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

var _ Publisher = &PubSubPublisher{}

// LogPublisher implements Publisher by logging events.
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
func (l *LogPublisher) Publish(ctx context.Context, event any) error {
	l.log.Info("Publishing event", zap.Any("event", event))
	return nil
}

// Close is a no-op for LogPublisher.
func (l *LogPublisher) Close() error { return nil }

var _ Publisher = &LogPublisher{}

// ParseTopicName parses a fully-qualified Pub/Sub topic name into project ID and topic ID.
func ParseTopicName(fullyQualifiedName string) (projectID, topicID string, err error) {
	if fullyQualifiedName == "@log" {
		return "", "", nil
	}
	// The expected format is "projects/PROJECT_ID/topics/TOPIC_ID"
	parts := strings.Split(fullyQualifiedName, "/")
	if len(parts) != 4 {
		return "", "", errs.New("invalid fully-qualified topic name format: %s", fullyQualifiedName)
	}

	if parts[0] != "projects" || parts[2] != "topics" {
		return "", "", errs.New("invalid fully-qualified topic name format: %s", fullyQualifiedName)
	}

	projectID = parts[1]
	topicID = parts[3]

	return projectID, topicID, nil
}
