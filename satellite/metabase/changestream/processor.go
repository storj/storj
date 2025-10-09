// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"storj.io/common/errs2"
)

var mon = monkit.Package()

// Processor processes change stream records in batches (parallel). This contains the logic to follow child partitions.
func Processor(ctx context.Context, log *zap.Logger, adapter Adapter, feedName string, startTime time.Time, fn func(record DataChangeRecord) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	tracker := &Tracker{
		todo:    make(map[string]Todo),
		status:  make(map[string]TodoStatus),
		receive: make(chan Todo),
	}

	eg, childCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case todoItem := <-tracker.receive:
				eg.Go(func() error {
					partitions, err := adapter.ChangeStream(childCtx, feedName, todoItem.Token, todoItem.StarTimestamp, func(record DataChangeRecord) error {
						return fn(record)
					})
					if err != nil {
						if !errs2.IsCanceled(err) && spanner.ErrCode(err) != codes.Canceled {
							tracker.Failed(todoItem.Token)
							tracker.NotifyReady()
						}
						//nolint
						log.Warn("failed to process partition (will be retried)", zap.String("token", todoItem.Token), zap.Error(err))
						return nil
					}
					for _, partition := range partitions {
						for _, children := range partition.ChildPartitions {
							tracker.Add(children.Token, children.ParentPartitionTokens, partition.StartTimestamp, partition.RecordSequence)
						}
					}
					tracker.Finish(todoItem.Token)
					tracker.NotifyReady()
					return nil
				})
			}
		}
	})
	tracker.Add("", nil, startTime, "")
	tracker.NotifyReady()
	return eg.Wait()
}

// Todo represents a partition to be processed.
type Todo struct {
	Token          string
	ParentTokens   []string
	StarTimestamp  time.Time
	RecordSequence string
}

// TodoStatus represents the processing status of a partition.
type TodoStatus int

const (
	statusReceived TodoStatus = iota
	statusRunning
	statusFinished
)

// Tracker tracks the processing status of partitions.
type Tracker struct {
	mu   sync.Mutex
	todo map[string]Todo
	// TODO: this one kept in memory forever. We should have a way to clean it up.
	status  map[string]TodoStatus
	receive chan Todo
}

// Add adds a new token to be tracked, and notify the listener, if new partitions are ready to be processed.
func (t *Tracker) Add(token string, parentTokens []string, start time.Time, recordSequence string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, found := t.status[token]; found {
		return
	}
	t.todo[token] = Todo{
		Token:          token,
		ParentTokens:   parentTokens,
		StarTimestamp:  start,
		RecordSequence: recordSequence,
	}
	t.status[token] = statusReceived
}

// NotifyReady checks for partitions that are ready to be processed and sends them to the receive channel.
func (t *Tracker) NotifyReady() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, todo := range t.todo {
		if t.status[todo.Token] != statusReceived {
			continue
		}
		if !allFinished(t.status, todo.ParentTokens) {
			continue
		}
		t.status[todo.Token] = statusRunning
		t.receive <- todo
	}
}

func allFinished(status map[string]TodoStatus, tokens []string) bool {
	for _, token := range tokens {
		if status[token] != statusFinished {
			return false
		}
	}
	return true
}

// Finish marks a token as finished.
func (t *Tracker) Finish(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status[token] = statusFinished
	delete(t.todo, token)
}

// Failed marks a token as failed, so it can be retried.
func (t *Tracker) Failed(token string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status[token] = statusReceived
}
