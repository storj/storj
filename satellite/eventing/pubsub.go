// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase/changestream"
)

// PublishMetadata holds observability metadata associated with a publish call.
// It is stored in the PendingResult and used for logging and eventkit events
// when delivery is confirmed or permanently fails.
type PublishMetadata struct {
	Log             *zap.Logger
	Timestamp       time.Time
	TransactionTag  string
	ProjectPublicID string
	BucketName      string
	EventName       string
	TopicName       string
	MessageSize     int64
}

// Publisher defines an interface for publishing events.
type Publisher interface {
	// Publish submits the event to the underlying backend and returns a
	// PendingResult for asynchronous delivery confirmation. meta carries
	// observability context (commit timestamp, bucket, topic, etc.) that is
	// stored in the PendingResult so the drain loop can emit events and
	// advance the partition watermark once delivery is confirmed or dropped.
	Publish(ctx context.Context, data []byte, meta PublishMetadata) changestream.PendingResult
	TopicName() string
	io.Closer
}

// pendingResult represents an in-flight publish operation whose delivery can
// be confirmed asynchronously. It implements changestream.PendingResult.
type pendingResult struct {
	meta   PublishMetadata
	result *pubsub.PublishResult
	called atomic.Bool
}

// Timestamp returns the commit timestamp of the change record that triggered
// this publish.
func (p *pendingResult) Timestamp() time.Time { return p.meta.Timestamp }

// Ready returns a channel that is closed when the Pub/Sub delivery result is ready.
func (p *pendingResult) Ready() <-chan struct{} { return p.result.Ready() }

// Get blocks until the message is confirmed delivered or the context is cancelled.
// Returns nil on success. Returns ctx.Err() if the context is cancelled.
// User configuration errors (deleted topic, missing permissions) are logged and
// dropped — they return nil so the drain loop advances the watermark without error.
func (p *pendingResult) Get(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !p.called.CompareAndSwap(false, true) {
		// This is a bug in the drain logic — Get must be called exactly once per result.
		// Log the caller's stack trace to help identify the double-call site.
		p.meta.Log.Warn("pendingResult.Get called more than once", zap.StackSkip("stack", 1))
		return nil
	}

	_, err = p.result.Get(ctx)
	if err == nil {
		ek.Event("publish_success",
			eventkit.String("transaction_tag", p.meta.TransactionTag),
			eventkit.String("project_public_id", p.meta.ProjectPublicID),
			eventkit.String("bucket", p.meta.BucketName),
			eventkit.String("event_name", p.meta.EventName),
			eventkit.String("topic", p.meta.TopicName),
			eventkit.Int64("message_size_bytes", p.meta.MessageSize),
			eventkit.Duration("latency", time.Since(p.meta.Timestamp)))
		return nil
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	userConfigErr := userConfigError(err)
	ek.Event("publish_failed",
		eventkit.String("transaction_tag", p.meta.TransactionTag),
		eventkit.String("project_public_id", p.meta.ProjectPublicID),
		eventkit.String("bucket", p.meta.BucketName),
		eventkit.String("event_name", p.meta.EventName),
		eventkit.String("topic", p.meta.TopicName),
		eventkit.Bool("user_config_error", userConfigErr),
		eventkit.String("error", err.Error()))

	if userConfigErr {
		p.meta.Log.Warn("Dropping event due to user configuration error",
			zap.String("project_public_id", p.meta.ProjectPublicID),
			zap.String("bucket_name", p.meta.BucketName),
			zap.String("topic", p.meta.TopicName),
			zap.Error(err))
		return nil
	}

	return err
}

// NewPublisher creates a new EventPublisher based on the provided topic name.
func NewPublisher(ctx context.Context, topicName string, opts ...option.ClientOption) (Publisher, error) {
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
	}, opts...)
}

// PubSubConfig holds configuration for Pub/Sub publisher.
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
func NewPubSubPublisher(ctx context.Context, cfg PubSubConfig, opts ...option.ClientOption) (_ *PubSubPublisher, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := pubsub.NewClient(ctx, cfg.ProjectID, opts...)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &PubSubPublisher{
		client:    client,
		publisher: client.Publisher(cfg.TopicID),
	}, nil
}

// Publish submits the event to the configured Pub/Sub topic and returns a
// PendingResult that can be used to confirm delivery asynchronously.
func (p *PubSubPublisher) Publish(ctx context.Context, data []byte, meta PublishMetadata) changestream.PendingResult {
	return &pendingResult{
		meta:   meta,
		result: p.publisher.Publish(ctx, &pubsub.Message{Data: data}),
	}
}

// TopicName returns the fully-qualified topic name for this publisher.
func (p *PubSubPublisher) TopicName() string {
	if p.publisher == nil {
		return ""
	}
	return p.publisher.String()
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

// Publish logs the event and returns a pre-resolved PendingResult.
func (l *LogPublisher) Publish(_ context.Context, data []byte, meta PublishMetadata) changestream.PendingResult {
	l.log.Info("Publishing event", zap.ByteString("data", data))
	return changestream.ImmediateResult(meta.Timestamp)
}

// TopicName returns the special topic name for log publishers.
func (l *LogPublisher) TopicName() string {
	return "@log"
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

// userConfigError returns true if the error is due to user
// misconfiguration such as deleted topic/project or missing permissions.
// These errors should not be retried.
func userConfigError(err error) bool {
	if err == nil {
		return false
	}

	s, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch s.Code() {
	case
		// GCP project deleted
		// rpc error: code = NotFound desc = Requested project not found or user does not have access to it
		// (project=non-existing-project). Make sure to specify the unique project identifier and not the
		// Google Cloud Console display name.
		//
		// GCP Pub/Sub topic deleted
		// rpc error: code = NotFound desc = Resource not found (resource=non-existing-topic).
		codes.NotFound,
		// GCP Pub/Sub topic removed the Pub/Sub Publisher role for the bucket-eventing service account
		// rpc error: code = PermissionDenied desc = User not authorized to perform this action.
		//
		// GCP account disabled due to billing or other issues.
		codes.PermissionDenied:
		return true
	default:
		return false
	}
}
