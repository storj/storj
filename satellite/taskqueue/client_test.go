// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testJob struct {
	NodeID  string
	PieceID string
	Data    []byte
	Retry   int
	Active  bool
	Score   float64
}

type taggedJob struct {
	ID   string `redis:"job_id"`
	Name string `redis:"job_name"`
	Skip string `redis:"-"`
	//lint:ignore U1000 testing unexported field handling
	hidden string
}

func TestMarshalUnmarshal(t *testing.T) {
	original := testJob{
		NodeID:  "node-abc",
		PieceID: "piece-xyz",
		Data:    []byte{0xde, 0xad, 0xbe, 0xef},
		Retry:   3,
		Active:  true,
		Score:   0.95,
	}

	fields, err := marshalStruct(original)
	require.NoError(t, err)

	assert.Equal(t, "node-abc", fields["nodeid"])
	assert.Equal(t, "piece-xyz", fields["pieceid"])
	assert.Equal(t, "deadbeef", fields["data"])
	assert.Equal(t, "3", fields["retry"])
	assert.Equal(t, "true", fields["active"])
	assert.Equal(t, "0.95", fields["score"])

	var dest testJob
	// convert to map[string]any as Redis returns
	values := make(map[string]any, len(fields))
	for k, v := range fields {
		values[k] = v
	}

	err = unmarshalStruct(values, &dest)
	require.NoError(t, err)
	assert.Equal(t, original, dest)
}

func TestMarshalTags(t *testing.T) {
	original := taggedJob{
		ID:   "123",
		Name: "test",
		Skip: "should-be-skipped",
	}

	fields, err := marshalStruct(original)
	require.NoError(t, err)

	assert.Equal(t, "123", fields["job_id"])
	assert.Equal(t, "test", fields["job_name"])
	_, hasSkip := fields["-"]
	assert.False(t, hasSkip)
	_, hasSkipName := fields["skip"]
	assert.False(t, hasSkipName)
}

func TestMarshalPointer(t *testing.T) {
	original := &testJob{NodeID: "node-1", Retry: 5}

	fields, err := marshalStruct(original)
	require.NoError(t, err)
	assert.Equal(t, "node-1", fields["nodeid"])
	assert.Equal(t, "5", fields["retry"])
}

func TestUnmarshalNonPointer(t *testing.T) {
	var dest testJob
	err := unmarshalStruct(map[string]any{}, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pointer to struct")
}

func TestMarshalNonStruct(t *testing.T) {
	_, err := marshalStruct("not a struct")
	require.Error(t, err)
}

func getRedisAddr(t *testing.T) string {
	addr := os.Getenv("STORJ_TEST_REDIS")
	if addr == "" {
		t.Skip("STORJ_TEST_REDIS not set, skipping integration test")
	}
	return addr
}

func TestPushPop(t *testing.T) {
	addr := getRedisAddr(t)
	ctx := context.Background()

	client, err := NewClient(ctx, Config{
		Address:  addr,
		Group:    "test-group",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	stream := "test-push-pop-" + t.Name()

	// clean up stream
	defer client.db.Del(ctx, stream)

	job := testJob{
		NodeID:  "node-1",
		PieceID: "piece-1",
		Data:    []byte{0x01, 0x02},
		Retry:   0,
		Active:  true,
		Score:   1.5,
	}

	err = client.Push(ctx, stream, job)
	require.NoError(t, err)

	var got testJob
	ok, err := client.Pop(ctx, stream, &got, time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, job, got)

	// stream should be empty now
	ok, err = client.Pop(ctx, stream, &got, 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPushBatch(t *testing.T) {
	addr := getRedisAddr(t)
	ctx := context.Background()

	client, err := NewClient(ctx, Config{
		Address:  addr,
		Group:    "test-group-batch",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	stream := "test-push-batch-" + t.Name()
	defer client.db.Del(ctx, stream)

	items := []any{
		testJob{NodeID: "n1", Retry: 1},
		testJob{NodeID: "n2", Retry: 2},
		testJob{NodeID: "n3", Retry: 3},
	}

	err = client.PushBatch(ctx, stream, items)
	require.NoError(t, err)

	for i, expected := range items {
		var got testJob
		ok, err := client.Pop(ctx, stream, &got, time.Second)
		require.NoError(t, err, "item %d", i)
		require.True(t, ok, "item %d", i)
		assert.Equal(t, expected.(testJob).NodeID, got.NodeID, "item %d", i)
		assert.Equal(t, expected.(testJob).Retry, got.Retry, "item %d", i)
	}
}

func TestPeek(t *testing.T) {
	addr := getRedisAddr(t)
	ctx := context.Background()

	client, err := NewClient(ctx, Config{
		Address:  addr,
		Group:    "test-group-peek",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	stream := "test-peek-" + t.Name()
	defer client.db.Del(ctx, stream)

	// peek on empty stream
	var got testJob
	ok, err := client.Peek(ctx, stream, &got)
	require.NoError(t, err)
	assert.False(t, ok)

	// push and peek
	job := testJob{NodeID: "peek-node", PieceID: "peek-piece"}
	err = client.Push(ctx, stream, job)
	require.NoError(t, err)

	ok, err = client.Peek(ctx, stream, &got)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "peek-node", got.NodeID)

	// peek again — message should still be there
	var got2 testJob
	ok, err = client.Peek(ctx, stream, &got2)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "peek-node", got2.NodeID)
}

func TestPushBatchEmpty(t *testing.T) {
	addr := getRedisAddr(t)
	ctx := context.Background()

	client, err := NewClient(ctx, Config{
		Address:  addr,
		Group:    "test-group-empty",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	err = client.PushBatch(ctx, "empty-stream", nil)
	require.NoError(t, err)
}
