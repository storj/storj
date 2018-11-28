// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCA(t *testing.T) {
	expectedDifficulty := uint16(4)

	ca, err := NewCA(context.Background(), NewCAOptions{
		Difficulty:  expectedDifficulty,
		Concurrency: 5,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, ca)

	actualDifficulty, err := ca.ID.Difficulty()
	assert.NoError(t, err)
	assert.True(t, actualDifficulty >= expectedDifficulty)
}

func TestFullCertificateAuthority_NewIdentity(t *testing.T) {
	check := func(err error, v interface{}) {
		if !assert.NoError(t, err) || !assert.NotEmpty(t, v) {
			t.Fail()
		}
	}

	ca, err := NewTestCA(context.Background())
	check(err, ca)
	fi, err := ca.NewIdentity()
	check(err, fi)

	assert.Equal(t, ca.Cert, fi.CA)
	assert.Equal(t, ca.ID, fi.ID)
	assert.NotEqual(t, ca.Key, fi.Key)
	assert.NotEqual(t, ca.Cert, fi.Leaf)

	err = fi.Leaf.CheckSignatureFrom(ca.Cert)
	assert.NoError(t, err)
}

func NewCABenchmark(b *testing.B, difficulty uint16, concurrency uint) {
	for i := 0; i < b.N; i++ {
		_, _ = NewCA(context.Background(), NewCAOptions{
			Difficulty:  difficulty,
			Concurrency: concurrency,
		})
	}
}

func BenchmarkNewCA_Difficulty8_Concurrency1(b *testing.B) {
	NewCABenchmark(b, 8, 1)
}

func BenchmarkNewCA_Difficulty8_Concurrency2(b *testing.B) {
	NewCABenchmark(b, 8, 2)
}

func BenchmarkNewCA_Difficulty8_Concurrency5(b *testing.B) {
	NewCABenchmark(b, 8, 5)
}

func BenchmarkNewCA_Difficulty8_Concurrency10(b *testing.B) {
	NewCABenchmark(b, 8, 10)
}
