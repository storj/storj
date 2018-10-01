// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
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

func TestEnqueueDequeue(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()

	seg := &pb.InjuredSegment{
		Path:       "abc",
		LostPieces: []int32{},
	}
	err := queue.Enqueue(seg)
	assert.NoError(t, err)

	s, err := queue.Dequeue()
	assert.NoError(t, err)
	assert.True(t, proto.Equal(&s, seg))
}

func TestDequeueEmptyQueue(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()
	s, err := queue.Dequeue()
	assert.Error(t, err)
	assert.Equal(t, pb.InjuredSegment{}, s)
}

func TestForceError(t *testing.T) {
	
}
