// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func randBucketname(n int) metabase.BucketName {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[testrand.Intn(len(letters))]
	}
	return metabase.BucketName(b)
}

func BenchmarkExpiredDeletion(b *testing.B) {
	if testing.Short() {
		expiredScenario{
			objects:           10,
			segmentsPerObject: 2,
			expiredRatio:      1,
		}.Run(b)
		return
	}
	expiredScenario{
		objects:           1000,
		segmentsPerObject: 10,
		expiredRatio:      0.001,
	}.Run(b)
}

type expiredScenario struct {
	objects           int
	segmentsPerObject int
	expiredRatio      float32
	// info filled in during execution.
	redundancy   storj.RedundancyScheme
	objectStream []metabase.ObjectStream
}

type expirationDateGenerator struct {
	expired    int
	nonExpired int
	now        time.Time
}

func (g *expirationDateGenerator) init(total int, ratio float32, now time.Time) {
	g.expired = int(ratio * float32(total))
	g.nonExpired = total - g.expired
	g.now = now
}

func (g *expirationDateGenerator) getDeadline() time.Time {
	timeInterval := time.Duration(testrand.Intn(36))*time.Hour + time.Hour
	if g.expired == 0 && g.nonExpired != 0 {
		g.nonExpired--
		return g.now.Add(timeInterval)
	}
	if g.nonExpired == 0 && g.expired != 0 {
		g.expired--
		return g.now.Add(-timeInterval)
	}
	expired := (testrand.Intn(2) == 0)
	if expired {
		g.expired--
		return g.now.Add(-timeInterval)
	}
	g.nonExpired--
	return g.now.Add(timeInterval)
}

// Run runs the scenario as a subtest.
func (s expiredScenario) Run(b *testing.B) {
	b.Run(s.name(), func(b *testing.B) { metabasetest.Bench(b, s.run) })
}

// name returns the scenario arguments as a string.
func (s *expiredScenario) name() string {
	return fmt.Sprintf("objects=%d,expiredRatio:%f", s.objects, s.expiredRatio)
}

// run runs the specified scenario.
//
//nolint:scopelint // This heavily uses loop variables without goroutines, avoiding these would add lots of boilerplate.
func (s *expiredScenario) run(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
	if s.redundancy.IsZero() {
		s.redundancy = storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			RequiredShares: 29,
			RepairShares:   50,
			OptimalShares:  85,
			TotalShares:    90,
			ShareSize:      256,
		}
	}

	nodes := make([]storj.NodeID, 10000)
	for i := range nodes {
		nodes[i] = testrand.NodeID()
	}

	now := time.Now()

	m := make(Metrics, 0, b.N)
	defer m.Report(b, "ns/loop")
	b.Run("Delete expired objects", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// wipe data so we can do the exact same test
			metabasetest.DeleteAll{}.Check(ctx, b, db)
			s.objectStream = nil
			var expiredGenerator expirationDateGenerator

			expiredGenerator.init(s.objects, s.expiredRatio, now)
			for objectIndex := 0; objectIndex < s.objects; objectIndex++ {

				expiresAt := expiredGenerator.getDeadline()
				objectStream := metabase.ObjectStream{
					ProjectID:  testrand.UUID(),
					BucketName: randBucketname(10),
					ObjectKey:  metabase.ObjectKey(testrand.UUID().String()),
					Version:    1,
					StreamID:   testrand.UUID(),
				}
				s.objectStream = append(s.objectStream, objectStream)
				_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
					ObjectStream: objectStream,
					Encryption: storj.EncryptionParameters{
						CipherSuite: storj.EncAESGCM,
						BlockSize:   256,
					},
					ExpiresAt: &expiresAt,
				})
				require.NoError(b, err)

				for segment := 0; segment < s.segmentsPerObject-1; segment++ {
					rootPieceID := testrand.PieceID()
					pieces := randPieces(int(s.redundancy.OptimalShares), nodes)

					err := db.BeginSegment(ctx, metabase.BeginSegment{
						ObjectStream: objectStream,
						Position: metabase.SegmentPosition{
							Part:  uint32(0),
							Index: uint32(segment),
						},
						RootPieceID: rootPieceID,
						Pieces:      pieces,
					})
					require.NoError(b, err)

					segmentSize := testrand.Intn(64*memory.MiB.Int()) + 1
					encryptedKey := testrand.BytesInt(storj.KeySize)
					encryptedKeyNonce := testrand.BytesInt(storj.NonceSize)

					err = db.CommitSegment(ctx, metabase.CommitSegment{
						ObjectStream: objectStream,
						Position: metabase.SegmentPosition{
							Part:  uint32(0),
							Index: uint32(segment),
						},
						EncryptedKey:      encryptedKey,
						EncryptedKeyNonce: encryptedKeyNonce,
						PlainSize:         int32(segmentSize),
						EncryptedSize:     int32(segmentSize),
						RootPieceID:       rootPieceID,
						Pieces:            pieces,
						Redundancy:        s.redundancy,
					})
					require.NoError(b, err)
				}

				_, err = db.CommitObject(ctx, metabase.CommitObject{
					ObjectStream: objectStream,
				})
				require.NoError(b, err)
			}

			m.Record(func() {
				err := db.DeleteExpiredObjects(ctx, metabase.DeleteExpiredObjects{
					ExpiredBefore: now,
				})
				require.NoError(b, err)
			})
		}
	})
	if len(s.objectStream) == 0 {
		b.Fatal("no objects uploaded")
	}
}
