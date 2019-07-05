// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

func TestAuditSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		type pathCount struct {
			path  storj.Path
			count int
		}

		// note: to simulate better,
		// change limit in library to 5 in
		// list api call, default is  0 == 1000 listing
		//populate metainfo with 10 non-expired pointers of test data
		tests, cursor, metainfo := populateTestData(t, planet, &timestamp.Timestamp{Seconds: time.Now().Unix() + 3000})

		t.Run("NextStripe", func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.bm, func(t *testing.T) {
					stripe, _, err := cursor.NextStripe(ctx)
					if err != nil {
						require.Error(t, err)
						require.Nil(t, stripe)
					} else {
						require.NotNil(t, stripe)
					}
				})
			}
		})

		// test to see how random paths are
		t.Run("probabilisticTest", func(t *testing.T) {
			list, _, err := metainfo.List(ctx, "", "", "", true, 10, meta.None)
			require.NoError(t, err)
			require.Len(t, list, 10)

			// get count of items picked at random
			uniquePathCounted := []pathCount{}
			pathCounter := []pathCount{}

			// get a list of 100 paths generated from random
			for i := 0; i < 100; i++ {
				pointerItem := list[testrand.Int63n(int64(len(list)))]
				path := pointerItem.Path
				val := pathCount{path: path, count: 1}
				pathCounter = append(pathCounter, val)
			}

			// get a count for paths in list
			for _, pc := range pathCounter {
				skip := false
				for i, up := range uniquePathCounted {
					if reflect.DeepEqual(pc.path, up.path) {
						up.count++
						uniquePathCounted[i] = up
						skip = true
						break
					}
				}
				if !skip {
					uniquePathCounted = append(uniquePathCounted, pc)
				}
			}

			// Section: binomial test for randomness
			n := float64(100) // events
			p := float64(.10) // theoretical probability of getting  1/10 paths
			m := n * p
			s := math.Sqrt(m * (1 - p)) // binomial distribution

			// if values fall outside of the critical values of test statistics (ie Z value)
			// in a 2-tail test
			// we can assume, 95% confidence, it's not sampling according to a 10% probability
			for _, v := range uniquePathCounted {
				z := (float64(v.count) - m) / s
				if z <= -1.96 || z >= 1.96 {
					t.Log(false)
				} else {
					t.Log(true)
				}
			}
		})
	})
}

func TestDeleteExpired(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		//populate metainfo with 10 expired pointers of test data
		_, cursor, metainfo := populateTestData(t, planet, &timestamp.Timestamp{})
		//make sure it they're in there
		list, _, err := metainfo.List(ctx, "", "", "", true, 10, meta.None)
		require.NoError(t, err)
		require.Len(t, list, 10)
		// make sure an error and no pointer is returned
		t.Run("NextStripe", func(t *testing.T) {
			stripe, _, err := cursor.NextStripe(ctx)
			require.NoError(t, err)
			require.Nil(t, stripe)
		})
		//make sure it they're not in there anymore
		list, _, err = metainfo.List(ctx, "", "", "", true, 10, meta.None)
		require.NoError(t, err)
		require.Len(t, list, 0)
	})
}

type testData struct {
	bm   string
	path storj.Path
}

func populateTestData(t *testing.T, planet *testplanet.Planet, expiration *timestamp.Timestamp) ([]testData, *audit.Cursor, *metainfo.Service) {
	ctx := context.TODO()
	tests := []testData{
		{bm: "success-1", path: "folder1/file1"},
		{bm: "success-2", path: "foodFolder1/file1/file2"},
		{bm: "success-3", path: "foodFolder1/file1/file2/foodFolder2/file3"},
		{bm: "success-4", path: "projectFolder/project1.txt/"},
		{bm: "success-5", path: "newProjectFolder/project2.txt"},
		{bm: "success-6", path: "Pictures/image1.png"},
		{bm: "success-7", path: "Pictures/Nature/mountains.png"},
		{bm: "success-8", path: "Pictures/City/streets.png"},
		{bm: "success-9", path: "Pictures/Animals/Dogs/dogs.png"},
		{bm: "success-10", path: "Nada/ビデオ/😶"},
	}
	metainfo := planet.Satellites[0].Metainfo.Service
	cursor := audit.NewCursor(metainfo)

	// put 10 pointers in db with expirations
	t.Run("putToDB", func(t *testing.T) {
		for _, tt := range tests {
			test := tt
			t.Run(test.bm, func(t *testing.T) {
				pointer := makePointer(test.path, expiration)
				require.NoError(t, metainfo.Put(ctx, test.path, pointer))
			})
		}
	})
	return tests, cursor, metainfo
}

func makePointer(path storj.Path, expiration *timestamp.Timestamp) *pb.Pointer {
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: 1,
		NodeId:   teststorj.NodeIDFromString("testId"),
	})
	return &pb.Pointer{
		ExpirationDate: expiration,
		Type:           pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			Redundancy: &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_RS,
				MinReq:           1,
				Total:            3,
				RepairThreshold:  2,
				SuccessThreshold: 3,
				ErasureShareSize: 2,
			},
			RootPieceId:  teststorj.PieceIDFromString("testId"),
			RemotePieces: rps,
		},
		SegmentSize: int64(10),
	}
}
