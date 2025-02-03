// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

func ExampleBall() {
	ball := NewBall()
	Provide[string](ball, func() string {
		return "test"
	})
	Provide[string](ball, func() string {
		return "test"
	})
	components := Find(ball, All)
	_ = components[0].Init(context.Background())
	fmt.Println(components[0].Instance())
}

func TestSortedDependency(t *testing.T) {
	ball := NewBall()
	Provide[DB](ball, NewDB)
	Provide[Service1](ball, NewService1)
	Provide[Service2](ball, NewService2)

	sorted := sortedComponents(ball)
	require.Equal(t, "storj.io/storj/shared/mud.DB", sorted[0].ID())
	require.Equal(t, "storj.io/storj/shared/mud.Service1", sorted[1].ID())
	require.Equal(t, "storj.io/storj/shared/mud.Service2", sorted[2].ID())
}

type Key struct{}

func TestView(t *testing.T) {
	ctx := testcontext.New(t)
	ball := NewBall()
	Supply[*DB](ball, &DB{status: "test"})
	View[*DB, DB](ball, Dereference[DB])
	// pointer registered, but value is received
	Provide[Key](ball, func(db DB) Key {
		return Key{}
	})
	err := ForEach(ball, func(component *Component) error {
		return component.Init(ctx)
	}, All)
	require.NoError(t, err)
}

func TestWrapper(t *testing.T) {
	ball := NewBall()
	Provide[DB](ball, NewDB)
	Provide[Service1](ball, NewService1, NewWrapper[DB](func(db DB) DB {
		return DB{
			status: "wrapped-" + db.status,
		}
	}))

	ctx := testcontext.New(t)

	err := ForEach(ball, func(component *Component) error {
		return component.Init(ctx)
	}, All)
	require.NoError(t, err)

	result, err := Execute[string](ctx, ball, func(service1 Service1) string {
		return service1.DB.status
	})
	require.NoError(t, err)
	require.Equal(t, "wrapped-auto", result)
}

func TestTags(t *testing.T) {
	ball := NewBall()
	Supply[*DB](ball, &DB{status: "test"})
	Tag[*DB, Tag1](ball, Tag1{
		Value: "ahoj",
	})

	tag, found := GetTag[*DB, Tag1](ball)
	require.True(t, found)
	require.Equal(t, "ahoj", tag.Value)

	Tag[*DB, Tag1](ball, Tag1{
		Value: "second",
	})
	tag, found = GetTag[*DB, Tag1](ball)
	require.True(t, found)
	require.Equal(t, "second", tag.Value)
}

func TestExecute(t *testing.T) {
	ctx := testcontext.New(t)
	ball := NewBall()
	Supply[string](ball, "Joe")
	result, err := Execute[string](ctx, ball, func(ctx context.Context, name string) string {
		if ctx != nil {
			return "hello " + name
		}
		// context was not injected
		return "error"
	})
	require.NoError(t, err)
	require.Equal(t, "hello Joe", result)
}

type Tag1 struct {
	Value string
}

type Tag2 struct {
	Value string
}

type T1 struct{}
type T2 struct{}
type T3 struct{}
type T4 struct{}
type T5 struct{}
type I interface{}

type DB struct {
	status string
}

func NewDB() DB {
	return DB{
		status: "auto",
	}
}

func (s DB) Close(ctx context.Context) error {
	fmt.Println("Closing DB")
	return nil
}

type Service interface {
	Run(ctx context.Context) error
	Close(ctx context.Context) error
}

type Service1 struct {
	DB DB
}

func (s Service1) Close(ctx context.Context) error {
	fmt.Println("Closing service 1")
	return nil
}

func (s Service1) Run(ctx context.Context) error {
	for i := 0; i < 20; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Println("running service1", i)
		time.Sleep(1 * time.Second)
	}
	return nil
}

func NewService1(db DB) Service1 {
	return Service1{
		DB: db,
	}
}

var _ Service = (*Service1)(nil)

type Service2 struct {
	Service1 Service1
}

func (s Service2) Close(ctx context.Context) error {
	fmt.Println("Closing service2")
	return nil
}

func (s Service2) Run(ctx context.Context) error {
	for i := 0; i < 10; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Println("running service2", i)
		time.Sleep(1 * time.Second)
	}
	return nil
}

func NewService2(service1 Service1) Service2 {
	return Service2{
		Service1: service1,
	}
}

var _ Service = (*Service2)(nil)

type Runnable interface {
	Run(ctx context.Context) error
}
