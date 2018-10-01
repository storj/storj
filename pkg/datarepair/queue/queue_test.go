
// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"testing"

	// "github.com/stretchr/testify/assert"

	// "storj.io/storj/pkg/pb"
	"storj.io/storj/storage/redis/redisserver"
)

func newTestQueue(t *testing.T) (*Queue, func()) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	queue, err := NewQueue(redisAddr, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	return queue, cleanup
}

func TestAdd(t *testing.T) {
	//TODO
}

func TestRemove(t *testing.T) {
	//TODO
}

func TestGetNext(t *testing.T) {
	//TODO
}

func TestGetSize(t *testing.T) {
	//TODO
}