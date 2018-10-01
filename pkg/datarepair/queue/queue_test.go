// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"testing"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/teststore"
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
	db := teststore.New()
	q := NewQueue(db)

	seg := &pb.InjuredSegment{
		Path:       "abc",
		LostPieces: []int32{},
	}
	err := q.Enqueue(seg)
	assert.NoError(t, err)

	s, err := q.Dequeue()
	assert.NoError(t, err)
	assert.True(t, proto.Equal(&s, seg))
}

func TestDequeueEmptyQueue(t *testing.T) {
	db := teststore.New()
	q := NewQueue(db)
	s, err := q.Dequeue()
	assert.Error(t, err)
	assert.Equal(t, pb.InjuredSegment{}, s)
}

func TestForceError(t *testing.T) {
	db := teststore.New()
	q := NewQueue(db)
	err := q.Enqueue(&pb.InjuredSegment{Path: "abc", LostPieces: []int32{}})
	assert.NoError(t, err)
	db.ForceError++
	item, err := q.Dequeue()
	assert.Equal(t, pb.InjuredSegment{}, item)
	assert.Error(t, err)
}

func TestSequential(t *testing.T) {
	db := teststore.New()
	q := NewQueue(db)
	const N = 100
	var addSegs []*pb.InjuredSegment
	var getSegs []*pb.InjuredSegment
	for i := 0; i < N; i++ {
		seg := &pb.InjuredSegment{
            Path:      	strconv.Itoa(i),
            LostPieces: []int32{int32(i)},
		}
		err := q.Enqueue(seg)
		assert.NoError(t, err)
		addSegs = append(addSegs, seg)
	}
	for i := 0; i < N; i++ {
		dqSeg, err := q.Dequeue()
		assert.NoError(t, err)
		getSegs = append(getSegs, &dqSeg)
	}
	for i := 0; i < N; i++ {
		assert.True(t, proto.Equal(addSegs[i], getSegs[i]))
	}
}
