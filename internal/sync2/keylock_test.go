// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"testing"

	"storj.io/storj/internal/sync2"
)

func TestKeyLock(t *testing.T) {
	ml := sync2.NewKeyLock()
	key := "hi"
	unlockFunc := ml.Lock(key)
	unlockFunc()
	unlockFunc = ml.RLock(key)
	unlockFunc()
}

func BenchmarkKeyLock(b *testing.B) {
	b.ReportAllocs()
	ml := sync2.NewKeyLock()
	for i := 0; i < b.N; i++ {
		unlockFunc := ml.Lock(i)
		unlockFunc()
	}
}
