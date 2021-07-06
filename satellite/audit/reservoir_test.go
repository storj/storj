// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
)

func TestReservoir(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	r := NewReservoir(3)

	seg := func(n byte) Segment { return Segment{StreamID: uuid.UUID{0: n}} }

	// if we sample 3 segments, we should record all 3
	r.Sample(rng, seg(1))
	r.Sample(rng, seg(2))
	r.Sample(rng, seg(3))

	require.Equal(t, r.Segments[:], []Segment{seg(1), seg(2), seg(3)})
}
