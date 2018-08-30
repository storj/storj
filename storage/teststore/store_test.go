package teststore

import (
	"testing"

	"storj.io/storj/storage/testsuite"
)

func Test(t *testing.T)      { testsuite.RunTests(t, New()) }
func Benchmark(b *testing.B) { testsuite.RunBenchmarks(b, New()) }
