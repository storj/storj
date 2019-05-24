// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"
)

func TestFailingAudit(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	if err != nil {
		panic(err)
	}

	modifiedShares := make([]infectious.Share, len(shares))
	for i := range shares {
		modifiedShares[i] = shares[i].DeepCopy()
	}

	modifiedShares[0].Data[1] = '!'
	modifiedShares[2].Data[0] = '#'
	modifiedShares[3].Data[1] = '!'
	modifiedShares[4].Data[0] = 'b'

	badPieceNums := []int{0, 2, 3, 4}

	ctx := context.Background()
	auditPkgShares := make(map[int]Share, len(modifiedShares))
	for i := range modifiedShares {
		auditPkgShares[modifiedShares[i].Number] = Share{
			PieceNum: modifiedShares[i].Number,
			Data:     append([]byte(nil), modifiedShares[i].Data...),
		}
	}

	pieceNums, correctedShares, err := auditShares(ctx, 8, 14, auditPkgShares)
	if err != nil {
		panic(err)
	}

	for i, num := range pieceNums {
		if num != badPieceNums[i] {
			t.Fatal("expected nums in pieceNums to be same as in badPieceNums")
		}
	}

	assert.Equal(t, shares, correctedShares)
}

func TestNotEnoughShares(t *testing.T) {
	const (
		required = 8
		total    = 14
	)

	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	err = f.Encode([]byte("hello, world! __"), output)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	auditPkgShares := make(map[int]Share, len(shares))
	for i := range shares {
		auditPkgShares[shares[i].Number] = Share{
			PieceNum: shares[i].Number,
			Data:     append([]byte(nil), shares[i].Data...),
		}
	}
	_, _, err = auditShares(ctx, 20, 40, auditPkgShares)
	assert.Contains(t, err.Error(), "infectious: must specify at least the number of required shares")
}
