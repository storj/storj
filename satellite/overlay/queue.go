// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// ErrEmptyQueue is returned when the queue is empty
var ErrEmptyQueue = errs.New("empty queue")

// selectedStorageNodeQueue is a queue for storing a selection of storage nodes that are used
// to upload files to. When a request comes in from an uplink to upload a file, we retrieve a list
// of storagenodes to use from the queue to reduce time processing this info in real time.
// The queue will always try to have maxSize storage node selections queued up at all times.
// Items in the queue older than the expiredLimit will be deleted.
type selectedStorageNodeQueue struct {
	log          *zap.Logger
	maxSize      int
	expiredLimit time.Time
	data         []storageNodeSelection
	ch           chan struct{}
}

func newQueue(log *zap.Logger, size int, expiredLimit time.Time) *selectedStorageNodeQueue {
	return &selectedStorageNodeQueue{
		log:          log,
		maxSize:      size,
		expiredLimit: expiredLimit,
		data:         []storageNodeSelection{},
		ch:           make(chan struct{}, size),
	}
}

type storageNodeSelection struct {
	nodes     []*NodeDossier
	createdAt time.Time
}

func (s *storageNodeSelection) isExpired(ctx context.Context, expiredLimit time.Time) bool {
	if s.createdAt.Sub(time.Now().UTC()) > time.Since(expiredLimit) {
		return true
	}
	return false
}

func (q *selectedStorageNodeQueue) init(ctx context.Context) error {
	select {
	case q.ch <- struct{}{}: // if we can add to the channel, then add one to the queue
		snSelect, err := q.selectNodes(ctx)
		if err != nil {
			return err
		}
		q.push(ctx, snSelect)
	default:
		// channel is full, meaning the queue is full so we are done
		return nil
	}
	return nil
}

// Run fills the queue initially and then makes sure it always full
func (q *selectedStorageNodeQueue) Run(ctx context.Context) error {
	err := q.init(ctx)
	if err != nil {
		return err
	}
	go q.continuallyFillQueue(ctx)
	return nil
}

func (q *selectedStorageNodeQueue) continuallyFillQueue(ctx context.Context) {
	for {
		// this blocks until there is space in the channel. When the channel is able to
		// accept more that means the queue has space so lets add more items to fill the queue
		q.ch <- struct{}{}
		snSelect, err := q.selectNodes(ctx)
		if err != nil {
			q.log.Error("selecting nodes for queue", zap.Error(err))
		}
		q.push(ctx, snSelect)
	}
}

func (q *selectedStorageNodeQueue) selectNodes(ctx context.Context) (_ storageNodeSelection, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: implement, fill the queue using the same code thats currently used in production to select
	// storage nodes: overlay.service.FindStorageNodesWithPreferences(ctx, req, &service.config.Node)
	return storageNodeSelection{}, nil
}

// Pop gets the next item from the queue
func (q *selectedStorageNodeQueue) Pop(ctx context.Context) (_ []*NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	// grab item out of queue
	item, err := q.pop(ctx)
	if err != nil {
		return []*NodeDossier{}, err
	}
	// if the current item is expired then get next item until one is not expired
	for item.isExpired(ctx, q.expiredLimit) {
		item, err = q.pop(ctx)
		if err != nil {
			return []*NodeDossier{}, err
		}
	}
	return item.nodes, nil
}

func (q *selectedStorageNodeQueue) pop(ctx context.Context) (_ storageNodeSelection, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(q.ch) < 1 {
		return storageNodeSelection{}, ErrEmptyQueue
	}
	nextItem := q.data[0]
	<-q.ch
	if len(q.ch) == 0 {
		q.data = []storageNodeSelection{}
		return nextItem, nil
	}
	q.data = q.data[1:]
	return nextItem, nil
}

func (q *selectedStorageNodeQueue) push(ctx context.Context, newitem storageNodeSelection) (err error) {
	defer mon.Task()(&ctx)(&err)
	// if the queue is full then remove first item
	// question: or do we want to return when the queue is full
	if len(q.ch) == q.maxSize {
		_, err := q.pop(ctx)
		if err != nil {
			return err
		}
	}
	// append to end
	q.data = append(q.data, newitem)
	return nil
}
