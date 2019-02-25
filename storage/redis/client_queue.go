// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"github.com/go-redis/redis"

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
		if err == redis.Nil {
			return nil, storage.ErrEmptyQueue.New("")
		}
		return nil, Error.New("dequeue error: %v", err)
	}
	return storage.Value(out), nil
}

// Peekqueue returns upto 1000 entries in the queue without removing
func (client *Queue) Peekqueue(limit int) ([]storage.Value, error) {
	cmd := client.db.LRange(queueKey, 0, int64(limit))
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
