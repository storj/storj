// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/lrucache"
)

// RetentionRemainderRecorderConfig is a configuration struct for the retention remainder recorder.
type RetentionRemainderRecorderConfig struct {
	CacheExpiration time.Duration `help:"expiration of the retention remainder recorder" default:"10m"`
	CacheCapacity   int           `help:"capacity of the retention remainder recorder" default:"10000"`
}

// RemainderProductInfo holds the subset of product pricing info needed for remainder charge calculation.
// This avoids the accounting package importing the payments package.
type RemainderProductInfo struct {
	ProductID                int32
	MinimumRetentionDuration time.Duration
}

// PricingConfig holds the pricing configuration needed for retention remainder charge calculation.
type PricingConfig struct {
	PlacementProductMap map[int]int32
	ProductPrices       map[int32]RemainderProductInfo
}

// RecordRemainderChargesParams contains parameters for recording retention remainder charges.
type RecordRemainderChargesParams struct {
	ProjectID       uuid.UUID
	ProjectPublicID uuid.UUID
	BucketName      string
	Placement       storj.PlacementConstraint
	ObjectsFunc     func() []metabase.DeleteObjectsInfo
	DeletedAt       time.Time
}

// RemainderChargeRecorder records retention remainder charges when objects are deleted
// before their minimum retention period expires.
type RemainderChargeRecorder struct {
	log                     *zap.Logger
	db                      RetentionRemainderDB
	productPrices           map[int32]RemainderProductInfo
	placementProductMap     map[int]int32
	entitlementsService     *entitlements.Service
	projectEntitlementCache *lrucache.ExpiringLRUOf[entitlements.ProjectFeatures]
}

// NewRemainderChargeRecorder creates a new RemainderChargeRecorder.
func NewRemainderChargeRecorder(
	log *zap.Logger,
	db RetentionRemainderDB,
	pricingConfig PricingConfig,
	entitlementsService *entitlements.Service,
	config RetentionRemainderRecorderConfig,
) *RemainderChargeRecorder {
	return &RemainderChargeRecorder{
		log:                 log,
		db:                  db,
		productPrices:       pricingConfig.ProductPrices,
		placementProductMap: pricingConfig.PlacementProductMap,
		entitlementsService: entitlementsService,
		projectEntitlementCache: lrucache.NewOf[entitlements.ProjectFeatures](lrucache.Options{
			Expiration: config.CacheExpiration,
			Capacity:   config.CacheCapacity,
		}),
	}
}

// Record computes and persists retention remainder charges for the given deleted objects.
func (r *RemainderChargeRecorder) Record(ctx context.Context, params RecordRemainderChargesParams) {
	var err error
	defer mon.Task()(&ctx)(&err)

	product, err := r.getProductForBucket(ctx, params.ProjectPublicID, params.Placement)
	if err != nil {
		r.log.Error("failed to get product for bucket for deletion remainder",
			zap.Stringer("project", params.ProjectID),
			zap.String("bucket", params.BucketName),
			zap.Error(err),
		)
		return
	}

	if product.MinimumRetentionDuration <= 0 {
		return
	}

	objects := params.ObjectsFunc()

	var charge *RetentionRemainderCharge
	for _, obj := range objects {
		// Only charge for committed objects.
		if obj.Status != metabase.CommittedUnversioned && obj.Status != metabase.CommittedVersioned {
			continue
		}

		storageTime := params.DeletedAt.Sub(obj.CreatedAt)
		if storageTime >= product.MinimumRetentionDuration {
			continue // Object was stored for full retention period.
		}

		if charge == nil {
			charge = &RetentionRemainderCharge{
				ProjectID:  params.ProjectID,
				BucketName: params.BucketName,
				DeletedAt:  params.DeletedAt,
				ProductID:  product.ProductID,
			}
		}

		remainderDuration := product.MinimumRetentionDuration - storageTime
		charge.RemainderByteHours += remainderDuration.Hours() * float64(obj.TotalEncryptedSize)
	}

	if charge == nil {
		return
	}

	err = r.db.Upsert(ctx, *charge)
	if err != nil {
		r.log.Error("failed to record deletion remainder charge",
			zap.Stringer("project_id", params.ProjectID),
			zap.String("bucket", params.BucketName),
			zap.Float64("remainder_byte_hours", charge.RemainderByteHours),
			zap.Error(err),
		)
	}
}

func (r *RemainderChargeRecorder) getProductForBucket(ctx context.Context, projectPublicID uuid.UUID, placement storj.PlacementConstraint) (*RemainderProductInfo, error) {
	defer mon.Task()(&ctx)(nil)

	defaultProductID := r.placementProductMap[int(placement)]
	defaultProduct := r.productPrices[defaultProductID]

	if r.entitlementsService == nil {
		// entitlements not enabled
		return &defaultProduct, nil
	}

	feats, err := r.projectEntitlementCache.Get(ctx, projectPublicID.String(), func() (entitlements.ProjectFeatures, error) {
		feats, err := r.entitlementsService.Projects().GetByPublicID(ctx, projectPublicID)
		if err != nil {
			if entitlements.ErrNotFound.Has(err) {
				r.log.Info("no entitlements found for project, using default product",
					zap.Stringer("public_project_id", projectPublicID),
				)
				return entitlements.ProjectFeatures{}, nil
			}
			return entitlements.ProjectFeatures{}, err
		}

		return feats, nil
	})
	if err != nil {
		return nil, err
	}

	productID := feats.PlacementProductMappings[placement]
	if product, ok := r.productPrices[productID]; ok {
		return &product, nil
	}

	return &defaultProduct, nil
}
