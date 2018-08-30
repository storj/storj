package teststore

import (
	"testing"

	"storj.io/storj/storage/testsuite"
)

func TestCommon(t *testing.T)      { testsuite.RunTests(t, New()) }
func BenchmarkCommon(b *testing.B) { testsuite.RunBenchmarks(b, New()) }
