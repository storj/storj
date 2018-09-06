// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storelogger

import (
	"testing"

	"storj.io/storj/storage/teststore"
	"storj.io/storj/storage/testsuite"

	"go.uber.org/zap"
)

func TestSuite(t *testing.T) {
	store := teststore.New()
	logged := New(zap.NewNop(), store)
	testsuite.RunTests(t, logged)
}

func BenchmarkSuite(b *testing.B) {
	store := teststore.New()
	logged := New(zap.NewNop(), store)
	testsuite.RunBenchmarks(b, logged)
}
