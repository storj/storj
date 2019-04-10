// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststore

import (
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage/testsuite"
)

func TestSuite(t *testing.T)      { testsuite.RunTests(t, New()) }
func BenchmarkSuite(b *testing.B) { testsuite.RunBenchmarks(b, New()) }

func TestStuff(t *testing.T) {

	// Stuff(t, "", "c002d1e330")
	// Stuff(t, "0", "c002b7a39")
	// Stuff(t, nil, "c001f0f180")
	ij := &pb.InjuredSegment{Path: "0", LostPieces: []int32{0}}
	b1, _ := proto.Marshal(ij)
	ij.Path = "0"
	b2, _ := proto.Marshal(ij)
	fmt.Printf("%x   vs     %x\n", b1, b2)

	//data, _ := hex.DecodeString("0a0130120100")
	//data, _ := hex.DecodeString("a0130120100")
	proto.Unmarshal(b2, ij)

	fmt.Printf("!!! %s\n", ij.Path)
	require.Equal(t, 0, 1)
}

// func Stuff(t *testing.T, path, bytes string) {
// 	ij := &pb.InjuredSegment{
// 		Path:       storj.Path(path),
// 		LostPieces: []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
// 	}

// 	b, err := proto.Marshal(ij)
// 	assert.NoError(t, err)

// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// require.Equal(t, b, data)
// }
