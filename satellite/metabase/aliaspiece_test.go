// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/metabase"
)

func TestAliasPieces(t *testing.T) {
	type test struct {
		in    metabase.AliasPieces
		bytes []byte
	}
	tests := []test{
		{in: nil, bytes: nil},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 1},
		}, bytes: []byte{1, 0b00001_000, 1}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 1},
			{Number: 3, Alias: 2},
		}, bytes: []byte{1, 0b00001_000, 1, 0b00001_010, 2}},
		{in: metabase.AliasPieces{
			{Number: 3, Alias: 2},
		}, bytes: []byte{1, 0b00001_011, 2}},
		{in: metabase.AliasPieces{
			{Number: 4, Alias: 2},
		}, bytes: []byte{1, 0b00001_100, 2}},
		{in: metabase.AliasPieces{
			{Number: 9, Alias: 2},
		}, bytes: []byte{1, 0b00000_111, 0b00001_010, 2}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 0xF8},
		}, bytes: []byte{1, 0b00001_000, 0xF8, 0x01}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 0xF808},
		}, bytes: []byte{1, 0b00001_000, 0x88, 0xf0, 0x03}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 0xF808ba},
		}, bytes: []byte{1, 0b00001_000, 0xba, 0x91, 0xe0, 0x07}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 0xA},
			{Number: 1, Alias: 0xB},
			{Number: 2, Alias: 0xC},
		}, bytes: []byte{1, 0b00011_000, 0xA, 0xB, 0xC}},
		{in: metabase.AliasPieces{
			{Number: 2, Alias: 0xA},
			{Number: 3, Alias: 0xB},
			{Number: 4, Alias: 0xC},
		}, bytes: []byte{1, 0b00011_010, 0xA, 0xB, 0xC}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 0xA},
			{Number: 1, Alias: 0xB},
			{Number: 2, Alias: 0xC},
			{Number: 7, Alias: 0xD},
			{Number: 8, Alias: 0xE},
			{Number: 9, Alias: 0xF},
		}, bytes: []byte{1,
			0b00011_000, 0xA, 0xB, 0xC,
			0b00011_100, 0xD, 0xE, 0xF,
		}},
		{in: metabase.AliasPieces{
			{Number: 0, Alias: 1}, {Number: 1, Alias: 2}, {Number: 2, Alias: 3}, {Number: 3, Alias: 4}, {Number: 4, Alias: 5}, {Number: 5, Alias: 6}, {Number: 6, Alias: 7}, {Number: 7, Alias: 8},
			{Number: 8, Alias: 9}, {Number: 9, Alias: 10}, {Number: 10, Alias: 11}, {Number: 11, Alias: 12}, {Number: 12, Alias: 13}, {Number: 13, Alias: 14}, {Number: 14, Alias: 15}, {Number: 15, Alias: 16},
			{Number: 16, Alias: 17}, {Number: 17, Alias: 18}, {Number: 18, Alias: 19}, {Number: 19, Alias: 20}, {Number: 20, Alias: 21}, {Number: 21, Alias: 22}, {Number: 22, Alias: 23}, {Number: 23, Alias: 24},
			{Number: 24, Alias: 25}, {Number: 25, Alias: 26}, {Number: 26, Alias: 27}, {Number: 27, Alias: 28}, {Number: 28, Alias: 29}, {Number: 29, Alias: 30}, {Number: 30, Alias: 31}, {Number: 31, Alias: 32},
			{Number: 32, Alias: 33}, {Number: 33, Alias: 34},
		}, bytes: []byte{1,
			0b11111_000, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
			0b00011_000, 32, 33, 34,
		}},
	}

	for i, test := range tests {
		bytes, err := test.in.Bytes()
		require.NoError(t, err, i)
		require.Equal(t, test.bytes, bytes, i)

		out := metabase.AliasPieces{}
		err = out.SetBytes(bytes)
		require.NoError(t, err, i)

		require.Equal(t, test.in, out, i)
	}
}

func TestAliasPieces_Large(t *testing.T) {
	aliases := make(metabase.AliasPieces, 0xFF)
	for offset := 1; offset < 18; offset++ {
		for i := range aliases {
			aliases[i].Number = uint16(i * offset)
			aliases[i].Alias = metabase.NodeAlias(i + 1)
		}

		bytes, err := aliases.Bytes()
		require.NoError(t, err)

		var result metabase.AliasPieces
		err = result.SetBytes(bytes)
		require.NoError(t, err)

		require.Equal(t, result, aliases)
	}
}

func TestAliasPieces_Errors(t *testing.T) {
	aliases := metabase.AliasPieces{
		{Number: 1, Alias: 1},
		{Number: 0, Alias: 2},
	}
	_, err := aliases.Bytes()
	require.EqualError(t, err, "metabase: alias pieces not ordered")

	duplicate := metabase.AliasPieces{
		{Number: 0, Alias: 1},
		{Number: 0, Alias: 2},
	}
	_, err = duplicate.Bytes()
	require.EqualError(t, err, "metabase: alias pieces not ordered")

	err = aliases.SetBytes([]byte{17})
	require.EqualError(t, err, "metabase: unknown alias pieces header: 17")

	err = aliases.SetBytes([]byte{1, 0xFF})
	require.EqualError(t, err, "metabase: invalid alias pieces data")
}

func BenchmarkAliasPiecesBytes(b *testing.B) {
	benchmarkAliasPiecesBytes(b, 50, 85, 90)
	benchmarkAliasPiecesBytes(b, 16, 37, 50)
}

func benchmarkAliasPiecesBytes(b *testing.B, repair, optimal, total int) {
	prefix := fmt.Sprintf("repair=%d,optimal=%d,total=%d", repair, optimal, total)

	b.Run(prefix+"/2byte", func(b *testing.B) {
		aliases := make(metabase.AliasPieces, optimal)
		for i := range aliases {
			aliases[i] = metabase.AliasPiece{
				Number: uint16(i),
				Alias:  metabase.NodeAlias(0xFF + i),
			}
		}
		benchmarkAliases(b, aliases)
	})

	b.Run(prefix+"/3byte", func(b *testing.B) {
		aliases := make(metabase.AliasPieces, optimal)
		for i := range aliases {
			aliases[i] = metabase.AliasPiece{
				Number: uint16(i),
				Alias:  metabase.NodeAlias(0xFFFF + i),
			}
		}
		benchmarkAliases(b, aliases)
	})

	b.Run(prefix+"/4byte", func(b *testing.B) {
		aliases := make(metabase.AliasPieces, optimal)
		for i := range aliases {
			aliases[i] = metabase.AliasPiece{
				Number: uint16(i),
				Alias:  metabase.NodeAlias(0xFFFFFF + i),
			}
		}
		benchmarkAliases(b, aliases)
	})

	b.Run(prefix+"/sim", func(b *testing.B) {
		totalBytes := int64(0)
		minBytes, maxBytes := int64(0xFFFFFF), int64(0)

		for k := 0; k < b.N; k++ {
			numPieces := repair + k%(optimal-repair)
			aliases := make(metabase.AliasPieces, numPieces)
			for i, n := range rand.Perm(total)[:numPieces] {
				aliases[i].Number = uint16(n)
				aliases[i].Alias = metabase.NodeAlias(0xFF + i)
			}
			sort.Slice(aliases, func(i, k int) bool {
				return aliases[i].Number < aliases[k].Number
			})
			bytes, err := aliases.Bytes()
			if err != nil {
				b.Fatal(err)
			}

			b := int64(len(bytes))
			totalBytes += b
			if b < minBytes {
				minBytes = b
			}
			if b > maxBytes {
				maxBytes = b
			}
		}

		b.ReportMetric(float64(totalBytes)/float64(b.N), "B/avg")
		b.ReportMetric(float64(minBytes), "B/min")
		b.ReportMetric(float64(maxBytes), "B/max")
	})
}

var sinkBytes []byte
var sinkValue any

func benchmarkAliases(b *testing.B, aliases metabase.AliasPieces) {
	b.Run("Bytes", func(b *testing.B) {
		for k := 0; k < b.N; k++ {
			data, err := aliases.Bytes()
			if err != nil {
				b.Fatal(err)
			}
			sinkBytes = data
		}
	})

	encodedData, err := aliases.Bytes()
	require.NoError(b, err)
	b.Run("SetBytes", func(b *testing.B) {
		b.ReportAllocs()
		var aliases metabase.AliasPieces
		for k := 0; k < b.N; k++ {
			err := aliases.SetBytes(encodedData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Value", func(b *testing.B) {
		for k := 0; k < b.N; k++ {
			data, err := aliases.Value()
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = data
		}
	})

	encodedBase64 := base64.StdEncoding.EncodeToString(encodedData)

	b.Run("DecodeSpanner", func(b *testing.B) {
		b.ReportAllocs()
		var aliases metabase.AliasPieces
		for k := 0; k < b.N; k++ {
			err := aliases.DecodeSpanner(encodedBase64)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.ReportMetric(float64(len(encodedData)), "B")
}
