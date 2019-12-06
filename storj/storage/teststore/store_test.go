// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststore

import (
	"testing"

	"storj.io/storj/storage/testsuite"
)

func TestSuite(t *testing.T)      { testsuite.RunTests(t, New()) }
func BenchmarkSuite(b *testing.B) { testsuite.RunBenchmarks(b, New()) }
