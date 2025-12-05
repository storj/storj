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
func (qm *QueueMap) GetQueue(placement storj.PlacementConstraint) (q *jobqueue.Queue, err error) {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	q, ok := qm.queues[placement]
	if !ok {
		q, err = qm.queueFactory(placement)
		if err != nil {
			return nil, err
		}
		err = q.Start()
		if err != nil {
			return nil, fmt.Errorf("could not start queue for placement %d: %w", placement, err)
		}
		qm.queues[placement] = q
	}
	return q, nil
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

// ChooseQueues returns a map of queues that match the given placement
// constraints. If includedPlacements is non-empty, only queues for the given
// placements are returned. If excludedPlacements is non-empty, queues for the
// given placements are excluded.
func (qm *QueueMap) ChooseQueues(includedPlacements, excludedPlacements []storj.PlacementConstraint) map[storj.PlacementConstraint]*jobqueue.Queue {
	qm.lock.Lock()
	defer qm.lock.Unlock()

	queues := maps.Clone(qm.queues)
	if len(includedPlacements) > 0 {
		newQueues := make(map[storj.PlacementConstraint]*jobqueue.Queue)
		for _, placement := range includedPlacements {
			if q, ok := queues[placement]; ok {
				newQueues[placement] = q
			}
		}
		queues = newQueues
	}
	for _, placement := range excludedPlacements {
		delete(queues, placement)
	}

	return queues
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
