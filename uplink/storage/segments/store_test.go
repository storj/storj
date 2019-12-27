// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/storj/uplink/storage/segments"
)

func TestCalcNeededNodes(t *testing.T) {
	for i, tt := range []struct {
		k, m, o, n int16
		needed     int32
	}{
		{k: 0, m: 0, o: 0, n: 0, needed: 0},
		{k: 1, m: 1, o: 1, n: 1, needed: 1},
		{k: 1, m: 1, o: 2, n: 2, needed: 2},
		{k: 1, m: 2, o: 2, n: 2, needed: 2},
		{k: 2, m: 3, o: 4, n: 4, needed: 3},
		{k: 2, m: 4, o: 6, n: 8, needed: 3},
		{k: 20, m: 30, o: 40, n: 50, needed: 25},
		{k: 29, m: 35, o: 80, n: 95, needed: 34},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rs := storj.RedundancyScheme{
			RequiredShares: tt.k,
			RepairShares:   tt.m,
			OptimalShares:  tt.o,
			TotalShares:    tt.n,
		}

		assert.Equal(t, tt.needed, segments.CalcNeededNodes(rs), tag)
	}
}
