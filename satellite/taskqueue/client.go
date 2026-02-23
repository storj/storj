// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package taskqueue provides a Redis Streams-backed task queue client.
package taskqueue

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	// Error is the error class for taskqueue.
	Error = errs.Class("taskqueue")

	mon = monkit.Package()
)

// Config holds configuration for the taskqueue Client.
type Config struct {
	Address  string `help:"redis URL for task queue" default:"redis://localhost:6379"`
	Group    string `help:"consumer group name" default:"taskqueue"`
	Consumer string `help:"consumer name within the group" default:"worker"`
}

// Client is a Redis Streams-backed task queue client supporting Push/Pop/Peek.
type Client struct {
	db       *redis.Client
	group    string
	consumer string

	initialized sync.Map // tracks which streams have consumer groups created
}

// NewClient creates a new taskqueue Client and verifies the connection.
func NewClient(ctx context.Context, cfg Config) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	opts, err := redis.ParseURL(cfg.Address)
	if err != nil {
		return nil, Error.New("invalid Redis URL: %v", err)
	}

	db := redis.NewClient(opts)

	if err := db.Ping(ctx).Err(); err != nil {
		_ = db.Close()
		return nil, Error.New("ping failed: %v", err)
	}

	return &Client{
		db:       db,
		group:    cfg.Group,
		consumer: cfg.Consumer,
	}, nil
}

// Close closes the underlying Redis connection.
func (c *Client) Close() error {
	return c.db.Close()
}

// Push adds a single item to the given stream. item must be a struct or pointer to struct.
func (c *Client) Push(ctx context.Context, streamID string, item any) (err error) {
	defer mon.Task()(&ctx)(&err)

	fields, err := marshalStruct(item)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(c.db.XAdd(ctx, &redis.XAddArgs{
		Stream: streamID,
		Values: fields,
	}).Err())
}

// PushBatch adds multiple items to the given stream using a pipeline.
func (c *Client) PushBatch(ctx context.Context, streamID string, items []any) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(items) == 0 {
		return nil
	}

	pipe := c.db.Pipeline()
	for _, item := range items {
		fields, err := marshalStruct(item)
		if err != nil {
			return Error.Wrap(err)
		}
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: streamID,
			Values: fields,
		})
	}

	_, err = pipe.Exec(ctx)
	return Error.Wrap(err)
}

// Pop reads the next message from the stream, unmarshals it into dest, acknowledges it,
// and deletes it. dest must be a pointer to a struct. Returns false if no message was
// available within the timeout.
func (c *Client) Pop(ctx context.Context, streamID string, dest any, timeout time.Duration) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := c.ensureGroup(ctx, streamID); err != nil {
		return false, err
	}

	streams, err := c.db.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{streamID, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, Error.Wrap(err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return false, nil
	}

	msg := streams[0].Messages[0]

	if err := unmarshalStruct(msg.Values, dest); err != nil {
		return false, Error.Wrap(err)
	}

	pipe := c.db.Pipeline()
	pipe.XAck(ctx, streamID, c.group, msg.ID)
	pipe.XDel(ctx, streamID, msg.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, Error.Wrap(err)
	}

	return true, nil
}

// Peek returns the oldest message in the stream without consuming it.
// dest must be a pointer to a struct. Returns false if the stream is empty.
func (c *Client) Peek(ctx context.Context, streamID string, dest any) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	msgs, err := c.db.XRange(ctx, streamID, "-", "+").Result()
	if err != nil {
		return false, Error.Wrap(err)
	}

	if len(msgs) == 0 {
		return false, nil
	}

	if err := unmarshalStruct(msgs[0].Values, dest); err != nil {
		return false, Error.Wrap(err)
	}

	return true, nil
}

// ensureGroup creates the consumer group for the stream if it hasn't been created yet.
func (c *Client) ensureGroup(ctx context.Context, streamID string) error {
	if _, ok := c.initialized.Load(streamID); ok {
		return nil
	}

	err := c.db.XGroupCreateMkStream(ctx, streamID, c.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return Error.Wrap(err)
	}

	c.initialized.Store(streamID, true)
	return nil
}

// fieldInfo holds metadata about a struct field for Redis mapping.
type fieldInfo struct {
	index int
	name  string
}

// getFieldInfos returns the Redis field mappings for a struct type.
func getFieldInfos(t reflect.Type) ([]fieldInfo, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	var infos []fieldInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name := strings.ToLower(f.Name)
		if tag, ok := f.Tag.Lookup("redis"); ok {
			if tag == "-" {
				continue
			}
			name = tag
		}

		infos = append(infos, fieldInfo{index: i, name: name})
	}

	return infos, nil
}

// marshalStruct converts a struct into a map[string]any suitable for XADD.
func marshalStruct(item any) (map[string]any, error) {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	infos, err := getFieldInfos(v.Type())
	if err != nil {
		return nil, err
	}

	fields := make(map[string]any, len(infos))
	for _, info := range infos {
		fv := v.Field(info.index)
		s, err := fieldToString(fv)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", info.name, err)
		}
		fields[info.name] = s
	}

	return fields, nil
}

// unmarshalStruct populates a struct pointer from a Redis stream entry's values.
func unmarshalStruct(values map[string]any, dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to struct, got %T", dest)
	}
	v = v.Elem()

	infos, err := getFieldInfos(v.Type())
	if err != nil {
		return err
	}

	for _, info := range infos {
		raw, ok := values[info.name]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			return fmt.Errorf("field %q: expected string value, got %T", info.name, raw)
		}
		if err := stringToField(v.Field(info.index), s); err != nil {
			return fmt.Errorf("field %q: %w", info.name, err)
		}
	}

	return nil
}

func fieldToString(v reflect.Value) (string, error) {
	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return hex.EncodeToString(v.Bytes()), nil
		}
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, v.Len())
			for i := range b {
				b[i] = byte(v.Index(i).Uint())
			}
			return hex.EncodeToString(b), nil
		}
	}
	return "", fmt.Errorf("unsupported type %s", v.Type())
}

func stringToField(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(n)
	case reflect.Float32:
		n, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return err
		}
		v.SetFloat(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		v.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			data, err := hex.DecodeString(s)
			if err != nil {
				return err
			}
			v.SetBytes(data)
			return nil
		}
		return fmt.Errorf("unsupported type %s", v.Type())
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			data, err := hex.DecodeString(s)
			if err != nil {
				return err
			}
			if len(data) != v.Len() {
				return fmt.Errorf("expected %d bytes, got %d", v.Len(), len(data))
			}
			for i, b := range data {
				v.Index(i).SetUint(uint64(b))
			}
			return nil
		}
		return fmt.Errorf("unsupported type %s", v.Type())
	default:
		return fmt.Errorf("unsupported type %s", v.Type())
	}
	return nil
}
