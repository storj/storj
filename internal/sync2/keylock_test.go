package sync2_test

import (
	"testing"

	"storj.io/storj/internal/sync2"
)

func TestKeyLock(t *testing.T) {
	ml := sync2.NewKeyLock()
	key := "hi"
	ml.Lock(key)
	ml.Unlock(key)
	ml.RLock(key)
	ml.RUnlock(key)
}

func BenchmarkKeyLock(b *testing.B) {
	b.ReportAllocs()
	ml := sync2.NewKeyLock()
	for i := 0; i < b.N; i++ {
		ml.Lock(i)
		ml.Unlock(i)
	}
}
