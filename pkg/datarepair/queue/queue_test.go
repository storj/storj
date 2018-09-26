// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/redis/redisserver"
)

func newTestQueue(t *testing.T) *Queue {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	queue, err := NewQueue(redisAddr, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	return queue
}

func TestAdd(t *testing.T) {
	queue := newTestQueue(t)
	seg := &pb.InjuredSegment{
		Path:       "abc",
		LostPieces: []int32{},
	}
	dateTime, err := queue.Add(seg)
	assert.NoError(t, err)
	key, err := queue.DB.Get(dateTime)
	assert.NoError(t, err)
	assert.NotNil(t, key)
}

func TestRemove(t *testing.T) {

}

func TestGetNext(t *testing.T) {
}

func TestGetSize(t *testing.T) {

}
