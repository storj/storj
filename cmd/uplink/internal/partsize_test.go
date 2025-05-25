// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/storj/cmd/uplink/internal"
)

func TestCpPartSize(t *testing.T) {
	// 10 GiB file, should return 1 GiB
	cfg, err := internal.CalculatePartSize(10*memory.GiB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    1 * memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// 10000 GB file, should return 1 GiB.
	cfg, err = internal.CalculatePartSize(10000*memory.GB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    1 * memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// 20000 GiB file, should return 2 GiB.
	cfg, err = internal.CalculatePartSize(20000*memory.GiB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    2 * memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// 10 TiB file, should return 1GiB + 64MiB.
	cfg, err = internal.CalculatePartSize(10*memory.TiB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    (1*memory.GiB + 64*memory.MiB).Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// 256 MiB file, should return single part and 1GiB.
	cfg, err = internal.CalculatePartSize(256*memory.MiB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    1 * memory.GiB.Int64(),
		SinglePart:  true,
		Parallelism: 1,
	}, cfg)

	// 256 MiB file, should return single part and 64MiB chunk.
	cfg, err = internal.CalculatePartSize(256*memory.MiB.Int64(), 64*memory.MiB.Int64(), 8)
	require.Error(t, err)
	require.Equal(t, "the specified chunk size 64.0 MiB is too small, requires 1.0 GiB or larger", err.Error())
	require.Equal(t, internal.PartSizeConfig{}, cfg)

	// 20001 GiB file, should return 2GiB + 64MiB.
	cfg, err = internal.CalculatePartSize(20001*memory.GiB.Int64(), 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    (2*memory.GiB + 64*memory.MiB).Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// should return 1GiB as requested.
	cfg, err = internal.CalculatePartSize(memory.GiB.Int64()*1300, memory.GiB.Int64(), 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// should return 1 GiB and error, since preferred is too low.
	cfg, err = internal.CalculatePartSize(1300*memory.GiB.Int64(), memory.MiB.Int64(), 8)
	require.Error(t, err)
	require.Equal(t, "the specified chunk size 1.0 MiB is too small, requires 1.0 GiB or larger", err.Error())
	require.Equal(t, internal.PartSizeConfig{}, cfg)

	// negative length should return asked for amount
	cfg, err = internal.CalculatePartSize(-1, 1*memory.GiB.Int64(), 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    1 * memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// negative length with no preferred size should return 1GiB
	cfg, err = internal.CalculatePartSize(-1, 0, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    1 * memory.GiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// Small requested size should be rounded to 64MiB.
	cfg, err = internal.CalculatePartSize(-1, 100, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    64 * memory.MiB.Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)

	// negative length should return asked for amount, but rounded to 64MiB
	cfg, err = internal.CalculatePartSize(-1, 1*memory.GiB.Int64()+1, 8)
	require.NoError(t, err)
	require.Equal(t, internal.PartSizeConfig{
		PartSize:    (1*memory.GiB + 64*memory.MiB).Int64(),
		SinglePart:  false,
		Parallelism: 8,
	}, cfg)
}
