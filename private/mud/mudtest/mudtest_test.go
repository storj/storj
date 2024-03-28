// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mudtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/mud"
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
