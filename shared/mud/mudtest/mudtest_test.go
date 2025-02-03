// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mudtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/mud"
)

func TestRun(t *testing.T) {
	var s1 *Service1
	Run[*Service2](t, func(ball *mud.Ball) {
		mud.Provide[*Service1](ball, NewService1)
		mud.Provide[*Service2](ball, NewService2)
	}, func(ctx context.Context, t *testing.T, target *Service2) {
		s1 = target.Dependency
	})
	require.True(t, s1.closed)
}

func BenchmarkRun(b *testing.B) {
	var s1 *Service1
	Run[*Service2](b, func(ball *mud.Ball) {
		mud.Provide[*Service1](ball, NewService1)
		mud.Provide[*Service2](ball, NewService2)
	}, func(ctx context.Context, b *testing.B, target *Service2) {
		s1 = target.Dependency
	})
	require.True(b, s1.closed)
}

func TestInterface(t *testing.T) {
	ct := &closeableThing{n: 27}
	var result int
	Run[closeableInterface](t, func(ball *mud.Ball) {
		mud.Provide[closeableInterface](ball, func() closeableInterface {
			return ct
		})
	}, func(ctx context.Context, t *testing.T, target closeableInterface) {
		result = target.(*closeableThing).n
	})
	require.Equal(t, 27, result)
	require.True(t, ct.closed)
}

type Service1 struct {
	closed bool
}

func NewService1() *Service1 {
	return &Service1{}
}

func (s *Service1) Close() error {
	s.closed = true
	return nil
}

type Service2 struct {
	Dependency *Service1
}

func NewService2(service1 *Service1) *Service2 {
	return &Service2{
		Dependency: service1,
	}
}

type closeableThing struct {
	closed bool
	n      int
}

func (c *closeableThing) Close() error {
	c.closed = true
	return nil
}

type closeableInterface interface {
	Close() error
}
