// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

func TestImplementationAllInit(t *testing.T) {
	ctx := testcontext.New(t)

	ball := NewBall()
	Provide[PostgresDB](ball, func() PostgresDB {
		return PostgresDB{}
	})
	Provide[CockroachDB](ball, func() CockroachDB {
		return CockroachDB{}
	})

	Implementation[[]DBAdapter, PostgresDB](ball)
	Implementation[[]DBAdapter, CockroachDB](ball)

	err := ForEach(ball, Initialize(ctx), All)
	require.NoError(t, err)

	// by default all dependencies are initialized, and usable
	err = Execute0(ctx, ball, func(impl []DBAdapter) {
		require.Len(t, impl, 2)
	})
	require.NoError(t, err)
}

func TestImplementationOneInit(t *testing.T) {
	ctx := testcontext.New(t)

	ball := NewBall()
	Provide[PostgresDB](ball, func() PostgresDB {
		return PostgresDB{}
	})
	Provide[CockroachDB](ball, func() CockroachDB {
		return CockroachDB{}
	})

	Implementation[[]DBAdapter, PostgresDB](ball)
	Implementation[[]DBAdapter, CockroachDB](ball)

	pg := MustLookupComponent[PostgresDB](ball)
	err := pg.Init(ctx)
	require.NoError(t, err)

	adapters := MustLookupComponent[[]DBAdapter](ball)

	// this init will use all the initialized dependencies (in our case, postgres only)
	err = adapters.Init(ctx)
	require.NoError(t, err)

	// Cockroach is not initialized, therefore it's an optional dependency.
	err = Execute0(ctx, ball, func(impl []DBAdapter) {
		require.Len(t, impl, 1)
		require.Equal(t, "postgres", impl[0].Name())
	})
	require.NoError(t, err)

}

type DBAdapter interface {
	Name() string
}
type PostgresDB struct{}

func (p PostgresDB) Name() string {
	return "postgres"
}

type CockroachDB struct{}

func (p CockroachDB) Name() string {
	return "cockroach"
}
