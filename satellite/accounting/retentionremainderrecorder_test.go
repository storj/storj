// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular/eventkit/eventkitspy"
)

type mockRetentionRemainderDB struct {
	upserted  []accounting.RetentionRemainderCharge
	upsertErr error
}

func (m *mockRetentionRemainderDB) Upsert(_ context.Context, charge accounting.RetentionRemainderCharge) error {
	m.upserted = append(m.upserted, charge)
	return m.upsertErr
}

func (m *mockRetentionRemainderDB) GetUnbilledCharges(_ context.Context, _ accounting.GetUnbilledChargesOptions) ([]accounting.RetentionRemainderCharge, *accounting.RetentionRemainderContinuationToken, error) {
	return nil, nil, nil
}

func (m *mockRetentionRemainderDB) MarkChargesAsBilled(_ context.Context, _ uuid.UUID, _, _ time.Time) error {
	return nil
}

func TestRemainderChargeRecorderEventkitEmission(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const (
		productID                int32         = 2
		minimumRetentionDuration time.Duration = 90 * 24 * time.Hour // 90 days
		placement                              = 0
	)

	pricingConfig := accounting.PricingConfig{
		PlacementProductMap: map[int]int32{
			placement: productID,
		},
		ProductPrices: map[int32]accounting.RemainderProductInfo{
			productID: {
				ProductID:                productID,
				MinimumRetentionDuration: minimumRetentionDuration,
			},
		},
	}

	mockDB := &mockRetentionRemainderDB{}

	t.Run("enabled", func(t *testing.T) {
		recorder := accounting.NewRemainderChargeRecorder(
			zaptest.NewLogger(t),
			mockDB,
			pricingConfig,
			nil,
			accounting.RetentionRemainderRecorderConfig{
				CacheExpiration:         10 * time.Minute,
				CacheCapacity:           100,
				EventkitTrackingEnabled: true,
			},
		)

		projectID := uuid.UUID{1, 2, 3, 4}
		projectPublicID := uuid.UUID{5, 6, 7, 8}
		bucketName := "test-bucket"
		deletedAt := time.Date(2026, 1, 15, 14, 23, 45, 0, time.UTC)
		// Object created 30 days before deletion — less than minimumRetentionDuration (90 days).
		createdAt := deletedAt.Add(-30 * 24 * time.Hour)

		eventkitspy.Clear()

		recorder.Record(ctx, accounting.RecordRemainderChargesParams{
			ProjectID:       projectID,
			ProjectPublicID: projectPublicID,
			BucketName:      bucketName,
			Placement:       placement,
			DeletedAt:       deletedAt,
			ObjectsFunc: func() []metabase.DeleteObjectsInfo {
				return []metabase.DeleteObjectsInfo{
					{
						CreatedAt:          createdAt,
						Status:             metabase.CommittedUnversioned,
						TotalEncryptedSize: 1024 * 1024, // 1 MiB
					},
				}
			},
		})

		var events []*eventkit.Event
		for _, event := range eventkitspy.GetEvents() {
			if event.Name == "retention_remainder_charge" {
				events = append(events, event)
			}
		}

		require.Len(t, events, 1, "expected exactly one retention_remainder_charge event")

		tags := make(map[string]any, len(events[0].Tags))
		for _, tag := range events[0].Tags {
			tags[tag.Key] = struct{}{}
		}

		require.Contains(t, tags, "public_project_id")
		require.Contains(t, tags, "bucket_name")
		require.Contains(t, tags, "deleted_at")
		require.Contains(t, tags, "remainder_byte_hours")
		require.Contains(t, tags, "product_id")
		require.Contains(t, tags, "event_type")
	})

	t.Run("disabled", func(t *testing.T) {
		recorder := accounting.NewRemainderChargeRecorder(
			zaptest.NewLogger(t),
			mockDB,
			pricingConfig,
			nil,
			accounting.RetentionRemainderRecorderConfig{
				CacheExpiration:         10 * time.Minute,
				CacheCapacity:           100,
				EventkitTrackingEnabled: false,
			},
		)

		projectID := uuid.UUID{1, 2, 3, 4}
		projectPublicID := uuid.UUID{5, 6, 7, 8}
		bucketName := "test-bucket"
		deletedAt := time.Date(2026, 1, 15, 14, 23, 45, 0, time.UTC)
		createdAt := deletedAt.Add(-30 * 24 * time.Hour)

		eventkitspy.Clear()

		recorder.Record(ctx, accounting.RecordRemainderChargesParams{
			ProjectID:       projectID,
			ProjectPublicID: projectPublicID,
			BucketName:      bucketName,
			Placement:       placement,
			DeletedAt:       deletedAt,
			ObjectsFunc: func() []metabase.DeleteObjectsInfo {
				return []metabase.DeleteObjectsInfo{
					{
						CreatedAt:          createdAt,
						Status:             metabase.CommittedUnversioned,
						TotalEncryptedSize: 1024 * 1024,
					},
				}
			},
		})

		for _, event := range eventkitspy.GetEvents() {
			require.NotEqual(t, "retention_remainder_charge", event.Name, "expected no retention_remainder_charge event when tracking is disabled")
		}
	})
}
