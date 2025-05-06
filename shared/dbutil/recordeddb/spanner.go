// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package recordeddb

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"

	"storj.io/storj/shared/flightrecorder"
)

// RecordedSpannerClient is a wrapper around spanner.Client that instruments every call via the flight recorder.
type RecordedSpannerClient struct {
	*spanner.Client

	recorder *flightrecorder.Box
}

// WrapSpannerClient takes ownership of the passed-in spanner.Client, wrapping it in a RecordedSpannerClient.
func WrapSpannerClient(client *spanner.Client, recorder *flightrecorder.Box) *RecordedSpannerClient {
	return &RecordedSpannerClient{
		Client:   client,
		recorder: recorder,
	}
}

// ReadOnlyTransaction wraps spanner.Client.ReadOnlyTransaction,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) ReadOnlyTransaction() *spanner.ReadOnlyTransaction {
	c.record()
	return c.Client.ReadOnlyTransaction()
}

// ReadWriteTransaction wraps spanner.Client.ReadWriteTransaction,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) ReadWriteTransaction(ctx context.Context, f func(context.Context, *spanner.ReadWriteTransaction) error) (time.Time, error) {
	c.record()
	return c.Client.ReadWriteTransaction(ctx, f)
}

// ReadWriteTransactionWithOptions wraps spanner.Client.ReadWriteTransactionWithOptions,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) ReadWriteTransactionWithOptions(ctx context.Context, f func(context.Context, *spanner.ReadWriteTransaction) error, options spanner.TransactionOptions) (spanner.CommitResponse, error) {
	c.record()
	return c.Client.ReadWriteTransactionWithOptions(ctx, f, options)
}

// Single wraps spanner.Client.Single,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) Single() *spanner.ReadOnlyTransaction {
	c.record()
	return c.Client.Single()
}

// BatchReadOnlyTransaction wraps spanner.Client.BatchReadOnlyTransaction,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) BatchReadOnlyTransaction(ctx context.Context, tb spanner.TimestampBound) (*spanner.BatchReadOnlyTransaction, error) {
	c.record()
	return c.Client.BatchReadOnlyTransaction(ctx, tb)
}

// BatchReadOnlyTransactionFromID wraps spanner.Client.BatchReadOnlyTransactionFromID,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) BatchReadOnlyTransactionFromID(tid spanner.BatchReadOnlyTransactionID) *spanner.BatchReadOnlyTransaction {
	c.record()
	return c.Client.BatchReadOnlyTransactionFromID(tid)
}

// Apply wraps spanner.Client.Apply,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) Apply(ctx context.Context, ms []*spanner.Mutation, opts ...spanner.ApplyOption) (time.Time, error) {
	c.record()
	return c.Client.Apply(ctx, ms, opts...)
}

// BatchWrite wraps spanner.Client.BatchWrite,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) BatchWrite(ctx context.Context, mgs []*spanner.MutationGroup) *spanner.BatchWriteResponseIterator {
	c.record()
	return c.Client.BatchWrite(ctx, mgs)
}

// BatchWriteWithOptions wraps spanner.Client.BatchWriteWithOptions,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) BatchWriteWithOptions(ctx context.Context, mgs []*spanner.MutationGroup, opts spanner.BatchWriteOptions) *spanner.BatchWriteResponseIterator {
	c.record()
	return c.Client.BatchWriteWithOptions(ctx, mgs, opts)
}

// PartitionedUpdate wraps spanner.Client.PartitionedUpdate,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) PartitionedUpdate(ctx context.Context, s spanner.Statement) (int64, error) {
	c.record()
	return c.Client.PartitionedUpdate(ctx, s)
}

// PartitionedUpdateWithOptions wraps spanner.Client.PartitionedUpdateWithOptions,
// adding flight recorder instrumentation.
func (c *RecordedSpannerClient) PartitionedUpdateWithOptions(ctx context.Context, s spanner.Statement, opts spanner.QueryOptions) (int64, error) {
	c.record()
	return c.Client.PartitionedUpdateWithOptions(ctx, s, opts)
}

func (c *RecordedSpannerClient) record() {
	if c.recorder == nil {
		return
	}

	c.recorder.Enqueue(flightrecorder.EventTypeDB, 1) // 1 to skip record call.
}
