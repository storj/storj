// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/loov/hrtime"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func Benchmark(b *testing.B) {
	if testing.Short() {
		scenario{projects: 1, objects: 1, parts: 1, segments: 2}.Run(b)
		return
	}
	scenario{projects: 2, objects: 50, parts: 2, segments: 5}.Run(b)
}

type scenario struct {
	projects int
	objects  int
	parts    int
	segments int

	// info filled in during execution.
	redundancy   storj.RedundancyScheme
	projectID    []uuid.UUID
	objectStream []metabase.ObjectStream
}

// Run runs the scenario as a subtest.
func (s scenario) Run(b *testing.B) {
	b.Run(s.name(), func(b *testing.B) { metabasetest.Bench(b, s.run) })
}

// name returns the scenario arguments as a string.
func (s *scenario) name() string {
	return fmt.Sprintf("projects=%d,objects=%d,parts=%d,segments=%d", s.projects, s.objects, s.parts, s.segments)
}

// run runs the specified scenario.
//
//nolint:scopelint // This heavily uses loop variables without goroutines, avoiding these would add lots of boilerplate.
func (s *scenario) run(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
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
	for i := 0; i < s.projects; i++ {
		s.projectID = append(s.projectID, testrand.UUID())
	}

	nodes := make([]storj.NodeID, 10000)
	for i := range nodes {
		nodes[i] = testrand.NodeID()
	}

	prefixes := make([]string, len(s.projectID))
	for i := range prefixes {
		prefixes[i] = testrand.Path()
	}

	b.Run("Upload", func(b *testing.B) {
		totalUpload := make(Metrics, 0, b.N*s.projects*s.objects)
		beginObject := make(Metrics, 0, b.N*s.projects*s.objects)
		beginSegment := make(Metrics, 0, b.N*s.projects*s.objects)
		commitRemoteSegment := make(Metrics, 0, b.N*s.projects*s.objects*s.parts*(s.segments-1))
		commitInlineSegment := make(Metrics, 0, b.N*s.projects*s.objects*s.parts*1)
		commitObject := make(Metrics, 0, b.N*s.projects*s.objects)

		defer totalUpload.Report(b, "ns/upl")
		defer beginObject.Report(b, "ns/bobj")
		defer beginSegment.Report(b, "ns/bseg")
		defer commitRemoteSegment.Report(b, "ns/crem")
		defer commitInlineSegment.Report(b, "ns/cinl")
		defer commitObject.Report(b, "ns/cobj")

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// wipe data so we can do the exact same test
			b.StopTimer()
			metabasetest.DeleteAll{}.Check(ctx, b, db)
			if err := db.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{Nodes: nodes}); err != nil {
				require.NoError(b, err)
			}
			b.StartTimer()

			s.objectStream = nil
			for i, projectID := range s.projectID {
				for objectIndex := 0; objectIndex < s.objects; objectIndex++ {
					objectStream := metabase.ObjectStream{
						ProjectID:  projectID,
						BucketName: "bucket",
						ObjectKey:  metabase.ObjectKey(prefixes[i] + "/" + testrand.UUID().String()),
						Version:    1,
						StreamID:   testrand.UUID(),
					}
					s.objectStream = append(s.objectStream, objectStream)

					totalUpload.Record(func() {
						beginObject.Record(func() {
							_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
								ObjectStream: objectStream,
								Encryption: storj.EncryptionParameters{
									CipherSuite: storj.EncAESGCM,
									BlockSize:   256,
								},
							})
							require.NoError(b, err)
						})

						for part := 0; part < s.parts; part++ {
							for segment := 0; segment < s.segments-1; segment++ {
								rootPieceID := testrand.PieceID()
								pieces := randPieces(int(s.redundancy.OptimalShares), nodes)

								beginSegment.Record(func() {
									err := db.BeginSegment(ctx, metabase.BeginSegment{
										ObjectStream: objectStream,
										Position: metabase.SegmentPosition{
											Part:  uint32(part),
											Index: uint32(segment),
										},
										RootPieceID: rootPieceID,
										Pieces:      pieces,
									})
									require.NoError(b, err)
								})

								segmentSize := testrand.Intn(64*memory.MiB.Int()) + 1
								encryptedKey := testrand.BytesInt(storj.KeySize)
								encryptedKeyNonce := testrand.BytesInt(storj.NonceSize)

								commitRemoteSegment.Record(func() {
									err := db.CommitSegment(ctx, metabase.CommitSegment{
										ObjectStream: objectStream,
										Position: metabase.SegmentPosition{
											Part:  uint32(part),
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
								})
							}

							segmentSize := testrand.Intn(4*memory.KiB.Int()) + 1
							inlineData := testrand.BytesInt(segmentSize)
							encryptedKey := testrand.BytesInt(storj.KeySize)
							encryptedKeyNonce := testrand.BytesInt(storj.NonceSize)

							commitInlineSegment.Record(func() {
								err := db.CommitInlineSegment(ctx, metabase.CommitInlineSegment{
									ObjectStream: objectStream,
									Position: metabase.SegmentPosition{
										Part:  uint32(part),
										Index: uint32(s.segments - 1),
									},
									InlineData:        inlineData,
									EncryptedKey:      encryptedKey,
									EncryptedKeyNonce: encryptedKeyNonce,
									PlainSize:         int32(segmentSize),
								})
								require.NoError(b, err)
							})
						}

						commitObject.Record(func() {
							_, err := db.CommitObject(ctx, metabase.CommitObject{
								ObjectStream: objectStream,
							})
							require.NoError(b, err)
						})
					})
				}
			}
		}
	})

	if len(s.objectStream) == 0 {
		b.Fatal("no objects uploaded")
	}

	b.Run("Iterate", func(b *testing.B) {
		m := make(Metrics, 0, b.N*s.projects)
		defer m.Report(b, "ns/proj")

		for i := 0; i < b.N; i++ {
			for _, projectID := range s.projectID {
				m.Record(func() {
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: "bucket",
						Pending:    false,
					}, func(ctx context.Context, it metabase.ObjectsIterator) error {
						var entry metabase.ObjectEntry
						for it.Next(ctx, &entry) {
						}
						return nil
					})
					require.NoError(b, err)
				})
			}
		}
	})

	b.Run("Iterate with prefix", func(b *testing.B) {
		m := make(Metrics, 0, b.N*s.projects)
		defer m.Report(b, "ns/proj")

		for i := 0; i < b.N; i++ {
			for i, projectID := range s.projectID {
				m.Record(func() {
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  projectID,
						BucketName: "bucket",
						Prefix:     metabase.ObjectKey(prefixes[i]),
						Pending:    false,
					}, func(ctx context.Context, it metabase.ObjectsIterator) error {
						var entry metabase.ObjectEntry
						for it.Next(ctx, &entry) {
						}
						return nil
					})
					require.NoError(b, err)
				})
			}
		}
	})

	b.Run("ListSegments", func(b *testing.B) {
		m := make(Metrics, 0, b.N*len(s.objectStream))
		defer m.Report(b, "ns/obj")

		for i := 0; i < b.N; i++ {
			for _, object := range s.objectStream {
				m.Record(func() {
					var cursor metabase.SegmentPosition
					for {
						result, err := db.ListSegments(ctx, metabase.ListSegments{
							StreamID: object.StreamID,
							Cursor:   cursor,
						})
						require.NoError(b, err)
						if !result.More {
							break
						}
						cursor = result.Segments[len(result.Segments)-1].Position
					}
				})
			}
		}
	})

	b.Run("GetObjectLastCommitted", func(b *testing.B) {
		m := make(Metrics, 0, b.N*len(s.objectStream))
		defer m.Report(b, "ns/obj")

		for i := 0; i < b.N; i++ {
			for _, object := range s.objectStream {
				m.Record(func() {
					_, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
						ObjectLocation: object.Location(),
					})
					require.NoError(b, err)
				})
			}
		}
	})

	b.Run("GetSegmentByPosition", func(b *testing.B) {
		m := make(Metrics, 0, b.N*len(s.objectStream)*s.parts*s.segments)
		defer m.Report(b, "ns/seg")

		for i := 0; i < b.N; i++ {
			for _, object := range s.objectStream {
				for part := 0; part < s.parts; part++ {
					for segment := 0; segment < s.segments; segment++ {
						m.Record(func() {
							_, err := db.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
								StreamID: object.StreamID,
								Position: metabase.SegmentPosition{
									Part:  uint32(part),
									Index: uint32(segment),
								},
							})
							require.NoError(b, err)
						})
					}
				}
			}
		}
	})

	b.Run("IterateObjectsAllVersionsWithStatus", func(b *testing.B) {
		m := make(Metrics, 0, b.N*len(s.objectStream)*s.parts*s.segments)
		defer m.Report(b, "ns/seg")

		for i := 0; i < b.N; i++ {
			for _, object := range s.objectStream {
				m.Record(func() {
					err := db.IterateObjectsAllVersionsWithStatus(ctx, metabase.IterateObjectsWithStatus{
						ProjectID:  object.ProjectID,
						BucketName: object.BucketName,
						Recursive:  true,
						BatchSize:  1,
						Cursor: metabase.IterateCursor{
							Key: object.ObjectKey,
						},
						Pending:               false,
						IncludeCustomMetadata: true,
						IncludeSystemMetadata: true,
					}, func(ctx context.Context, it metabase.ObjectsIterator) error {
						var item metabase.ObjectEntry
						for it.Next(ctx, &item) {
						}
						return nil
					})
					require.NoError(b, err)
				})
			}
		}
	})
}

// Metrics records a set of time.Durations.
type Metrics []time.Duration

// Record records a single value to the slice.
func (m *Metrics) Record(fn func()) {
	start := hrtime.Now()
	fn()
	*m = append(*m, hrtime.Since(start))
}

// Report reports the metric with the specified name.
func (m *Metrics) Report(b *testing.B, name string) {
	hist := hrtime.NewDurationHistogram(*m, &hrtime.HistogramOptions{
		BinCount:        1,
		NiceRange:       true,
		ClampMaximum:    0,
		ClampPercentile: 0.999,
	})
	b.ReportMetric(hist.P50, name)
}

// randPieces returns randomized pieces.
func randPieces(count int, nodes []storj.NodeID) metabase.Pieces {
	pieces := make(metabase.Pieces, count)
	for i := range pieces {
		pieces[i] = metabase.Piece{
			Number: uint16(i),
			// TODO: this will rarely end up with duplicates in the segment,
			// however, it should be fine.
			StorageNode: nodes[testrand.Intn(len(nodes))],
		}
	}
	return pieces
}
