// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/testsuite"
)

func TestSuite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redis, err := redisserver.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { require.NoError(t, redis.Close()) }()

	client, err := NewClient(redis.Addr(), "", 1)
	if err != nil {
		t.Fatal(err)
	}

	client.SetLookupLimit(500)
	testsuite.RunTests(t, client)
}

func TestInvalidConnection(t *testing.T) {
	_, err := NewClient("", "", 1)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func BenchmarkSuite(b *testing.B) {
	ctx := context.Background()

	redis, err := redisserver.Start(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { require.NoError(b, redis.Close()) }()

	client, err := NewClient(redis.Addr(), "", 1)
	if err != nil {
		b.Fatal(err)
	}
	testsuite.RunBenchmarks(b, client)
}
