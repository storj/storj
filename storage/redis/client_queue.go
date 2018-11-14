// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"storj.io/storj/storage"
)

// Queue is the aliased entrypoint into Redis
type Queue Client

const queueKey = "queue"

// NewQueue returns a configured Client instance, verifying a successful connection to redis
func NewQueue(address, password string, db int) (*Queue, error) {
	queue, err := NewClient(address, password, db)
	return (*Queue)(queue), err
}

// NewQueueFrom returns a configured Client instance from a redis address, verifying a successful connection to redis
func NewQueueFrom(address string) (*Queue, error) {
	queue, err := NewClientFrom(address)
	return (*Queue)(queue), err
}

// Close closes a redis client
func (client *Queue) Close() error {
	return client.db.Close()
}

//Enqueue add a FIFO element, for the storage.Queue interface
func (client *Queue) Enqueue(value storage.Value) error {
	err := client.db.LPush(queueKey, []byte(value)).Err()
	if err != nil {
		return Error.New("enqueue error: %v", err)
	}
	return nil
}

//Dequeue removes a FIFO element, for the storage.Queue interface
func (client *Queue) Dequeue() (storage.Value, error) {
	out, err := client.db.RPop(queueKey).Bytes()
	if err != nil {
		return nil, Error.New("dequeue error: %v", err)
	}
	return storage.Value(out), nil
}

// Dequeue returns the next repair segement and removes it from the queue
func (client *Queue) Peekqueue() ([]storage.Value, error) {
	cmd := client.db.LRange(queueKey, 0, -1)
	items, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	result := make([]storage.Value, 0)
	for _, v := range items {
		result = append(result, storage.Value([]byte(v)))
	}
	return result, err
}
