// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"testing"

	"context"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCA(t *testing.T) {
	expectedDifficulty := uint16(4)

	ca, err := GenerateCA(context.Background(), expectedDifficulty, 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, ca)

	actualDifficulty := ca.Difficulty()
	assert.True(t, actualDifficulty >= expectedDifficulty)
}

// func tempCAConfig() (*CAConfig, func(), error) {
// 	tmpDir, err := ioutil.TempDir("", "tempCAConfig")
// 	if err != nil {
// 		return nil, nil, err
// 	}
//
// 	cleanup := func() { os.RemoveAll(tmpDir) }
//
// 	return &CAConfig{
// 		CertPath:   filepath.Join(tmpDir, "ca.cert"),
// 		KeyPath:    filepath.Join(tmpDir, "ca.key"),
// 		Difficulty: 0,
// 	}, cleanup, nil
// }
//
// func TestCAConfig_Stat(t *testing.T) {
// 	ca := GenerateCA(nil, 0, 5)
// 	assert.NotEmpty(t, ca)
//
// 	caC, done, err := tempCAConfig()
// }

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
