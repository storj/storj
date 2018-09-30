// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/redis"

)

func newTestQueue(t *testing.T) (*Queue, func()) {
	addr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	client, err := redis.NewClient(addr, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	queue := NewQueue(client)
	return queue, cleanup
}

func TestAdd(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()

	seg := &pb.InjuredSegment{
		Path:       "abc",
		LostPieces: []int32{},
	}
	key, err := queue.Add(seg)
	assert.NoError(t, err)
	val, err := queue.db.Get(key)
	assert.NoError(t, err)
	assert.NotNil(t, val)
}

func TestRemove(t *testing.T) {

}

func TestGetNext(t *testing.T) {
}

func TestGetSize(t *testing.T) {

}
