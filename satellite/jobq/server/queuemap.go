// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"storj.io/common/storj"
	"storj.io/storj/satellite/jobq/jobqueue"
)

// QueueMap is a thread-safe mapping of placement constraints to queues.
type QueueMap struct {
	log          *zap.Logger
	queues       map[storj.PlacementConstraint]*jobqueue.Queue
	lock         sync.Mutex
	queueFactory func(storj.PlacementConstraint) (*jobqueue.Queue, error)
}

// NewQueueMap creates a new QueueMap.
func NewQueueMap(log *zap.Logger, queueFactory func(storj.PlacementConstraint) (*jobqueue.Queue, error)) *QueueMap {
	return &QueueMap{
		log:          log,
		queues:       make(map[storj.PlacementConstraint]*jobqueue.Queue),
		queueFactory: queueFactory,
	}
}

// GetQueue gets the queue for the given placement. If no queue exists for the
// given placement, nil is returned.
func (qm *QueueMap) GetQueue(placement storj.PlacementConstraint) *jobqueue.Queue {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	return qm.queues[placement]
}

// GetAllQueues gets a copy of the current queue map. It is possible for another
// caller to have destroyed one or more queues between this call and the time
// when the caller uses the returned map. If this happens, the affected queues
// will simply appear empty.
func (qm *QueueMap) GetAllQueues() map[storj.PlacementConstraint]*jobqueue.Queue {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	return maps.Clone(qm.queues)
}

// AddQueue creates a new queue for the given placement. If a queue already
// exists for the given placement, an error is returned.
func (qm *QueueMap) AddQueue(placement storj.PlacementConstraint) error {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	if _, ok := qm.queues[placement]; ok {
		return fmt.Errorf("queue for placement %d already exists", placement)
	}
	newQueue, err := qm.queueFactory(placement)
	if err != nil {
		return fmt.Errorf("placement %d: %w", placement, err)
	}
	if err := newQueue.Start(); err != nil {
		return fmt.Errorf("programming error: %w", err)
	}
	qm.queues[placement] = newQueue
	return nil
}

// DestroyQueue destroys the queue for the given placement. If no queue exists
// for the given placement, an error is returned.
func (qm *QueueMap) DestroyQueue(placement storj.PlacementConstraint) error {
	qm.lock.Lock()
	q, ok := qm.queues[placement]
	if ok {
		delete(qm.queues, placement)
	}
	qm.lock.Unlock()

	if !ok {
		return fmt.Errorf("no queue for placement %d", placement)
	}
	q.Destroy()
	return nil
}

// StopAll stops and removes all queues.
func (qm *QueueMap) StopAll() {
	qm.lock.Lock()
	queues := qm.queues
	qm.queues = nil
	qm.lock.Unlock()

	for _, q := range queues {
		q.Destroy()
	}
}
