// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCA(t *testing.T) {
	const expectedDifficulty = 4

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
	ca, err := NewCA(context.Background(), NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	})

	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, ca)

	fi, err := ca.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, fi)

	assert.Equal(t, ca.Cert, fi.CA)
	assert.Equal(t, ca.ID, fi.ID)
	assert.NotEqual(t, ca.Key, fi.Key)
	assert.NotEqual(t, ca.Cert, fi.Leaf)

	err = fi.Leaf.CheckSignatureFrom(ca.Cert)
	assert.NoError(t, err)
}

func TestFullCAConfig_Save(t *testing.T) {
	// TODO(bryanchriswhite): test with both
	// TODO(bryanchriswhite): test with only cert path
	// TODO(bryanchriswhite): test with only key path
	t.FailNow()
}

func BenchmarkNewCA(b *testing.B) {
	ctx := context.Background()
	for _, difficulty := range []uint16{8, 12} {
		for _, concurrency := range []uint{1, 2, 5, 10} {
			test := fmt.Sprintf("%d/%d", difficulty, concurrency)
			b.Run(test, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = NewCA(ctx, NewCAOptions{
						Difficulty:  difficulty,
						Concurrency: concurrency,
					})
				}
			})
		}
	}
}
