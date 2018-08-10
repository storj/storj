// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCA(t *testing.T) {
	expectedDifficulty := uint16(4)

	ca, err := GenerateCA(context.Background(), expectedDifficulty, 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, ca)

	actualDifficulty := ca.ID.Difficulty()
	assert.True(t, actualDifficulty >= expectedDifficulty)
}

func BenchmarkGenerateCA_Difficulty8_Concurrency1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		GenerateCA(nil, expectedDifficulty, 1)
	}
}

func BenchmarkGenerateCA_Difficulty8_Concurrency2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		GenerateCA(nil, expectedDifficulty, 2)
	}
}

func BenchmarkGenerateCA_Difficulty8_Concurrency5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		GenerateCA(nil, expectedDifficulty, 5)
	}
}

func BenchmarkGenerateCA_Difficulty8_Concurrency10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		expectedDifficulty := uint16(8)
		GenerateCA(nil, expectedDifficulty, 10)
	}
}
