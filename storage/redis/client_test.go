// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"testing"

	"storj.io/storj/storage"
)

type RedisClientTest struct {
	*testing.T
	c storage.KeyValueStore
}

func NewRedisClientTest(t *testing.T) *RedisClientTest {
	c, err := NewClient("localhost:0", "", 0)
	return &RedisClientTest{
		T: t,
		c: c,
	}
}
