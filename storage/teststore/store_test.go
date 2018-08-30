package teststore

import (
	"testing"

	"storj.io/storj/storage"
)

func TestCommon(t *testing.T)      { storage.RunTests(t, New()) }
func BenchmarkCommon(b *testing.B) { storage.RunBenchmarks(b, New()) }
