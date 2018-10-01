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

func TestEnqueue(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()

	seg := &pb.InjuredSegment{
		Path:       "abc",
		LostPieces: []int32{},
	}
	err := queue.Enqueue(seg)
	assert.NoError(t, err)
}

func TestDequeue(t *testing.T) {
	//TODO
}