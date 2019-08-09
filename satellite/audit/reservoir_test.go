// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"github.com/stretchr/testify/require"
	"storj.io/storj/pkg/storj"
	"testing"
)

func TestReservoirSampling(t *testing.T) {
	reservoir := NewReservoir(1)

	path1 := storj.Path("test/path1")
	path2 := storj.Path("test/path2")
	path3 := storj.Path("test/path3")

	reservoir.sample(path1)
	require.Equal(t, 1, len(reservoir.Paths))

	reservoir.sample(path2)
	reservoir.sample(path3)
	require.Equal(t, 1, len(reservoir.Paths))

	// todo: ... not really sure how/what to test at this level
}
