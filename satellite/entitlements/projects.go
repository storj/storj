// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import (
	"context"
	"encoding/json"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ProductPlacementMappings maps product IDs to their corresponding placement constraints.
type ProductPlacementMappings map[int32][]storj.PlacementConstraint

// ProjectFeatures defines the features available for a project.
type ProjectFeatures struct {
	NewBucketPlacements      []storj.PlacementConstraint `json:"new_bucket_placements,omitempty"`
	ProductPlacementMappings ProductPlacementMappings    `json:"product_placement_mappings,omitempty"`
}

// Projects separates project-related entitlements functionality.
type Projects struct {
	service *Service
}

// GetByPublicID retrieves the features of a project by its public ID.
func (p *Projects) GetByPublicID(ctx context.Context, publicID uuid.UUID) (feats ProjectFeatures, err error) {
	defer mon.Task()(&ctx)(&err)

	ent, err := p.service.db.GetByScope(ctx, ConvertPublicIDToProjectScope(publicID))
	if err != nil {
		return ProjectFeatures{}, Error.Wrap(err)
	}

	err = json.Unmarshal(ent.Features, &feats)
	return feats, Error.Wrap(err)
}

// SetNewBucketPlacementsByPublicID sets the new bucket placement constraints for a project by its public ID.
func (p *Projects) SetNewBucketPlacementsByPublicID(ctx context.Context, publicID uuid.UUID, newBucketPlacements []storj.PlacementConstraint) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(newBucketPlacements) == 0 {
		return Error.New("placements cannot be empty")
	}

	scope := ConvertPublicIDToProjectScope(publicID)

	// Load current record (may not exist yet).
	ent, err := p.getEntitlementBeforeSet(ctx, scope)
	if err != nil {
		return Error.Wrap(err)
	}

	var features ProjectFeatures
	if len(ent.Features) > 0 {
		if err = json.Unmarshal(ent.Features, &features); err != nil {
			return Error.Wrap(err)
		}
	}
	features.NewBucketPlacements = newBucketPlacements

	return Error.Wrap(p.upsertNewEntitlement(ctx, ent, features))
}

// SetProductPlacementMappingsByPublicID sets the product placement mappings for a project by its public ID.
func (p *Projects) SetProductPlacementMappingsByPublicID(ctx context.Context, publicID uuid.UUID, newMappings ProductPlacementMappings) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newMappings == nil {
		return Error.New("product:placements mappings cannot be empty")
	}

	scope := ConvertPublicIDToProjectScope(publicID)

	// Load current record (may not exist yet).
	ent, err := p.getEntitlementBeforeSet(ctx, scope)
	if err != nil {
		return Error.Wrap(err)
	}

	var features ProjectFeatures
	if len(ent.Features) > 0 {
		if err = json.Unmarshal(ent.Features, &features); err != nil {
			return Error.Wrap(err)
		}
	}
	features.ProductPlacementMappings = newMappings

	return Error.Wrap(p.upsertNewEntitlement(ctx, ent, features))
}

// DeleteByPublicID removes the project entitlements by its public ID.
func (p *Projects) DeleteByPublicID(ctx context.Context, publicID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(p.service.db.DeleteByScope(ctx, ConvertPublicIDToProjectScope(publicID)))
}

func (p *Projects) getEntitlementBeforeSet(ctx context.Context, scope []byte) (ent *Entitlement, err error) {
	ent, err = p.service.db.GetByScope(ctx, scope)
	if err != nil {
		if !ErrNotFound.Has(err) {
			return nil, err
		}
		ent = &Entitlement{Scope: scope}
	}

	return ent, nil
}

func (p *Projects) upsertNewEntitlement(ctx context.Context, ent *Entitlement, feats ProjectFeatures) error {
	featsBytes, err := json.Marshal(feats)
	if err != nil {
		return err
	}
	ent.Features = featsBytes
	ent.UpdatedAt = time.Now()

	_, err = p.service.db.UpsertByScope(ctx, ent)
	return err
}

// ConvertPublicIDToProjectScope converts a public project ID to a database project scope.
func ConvertPublicIDToProjectScope(publicID uuid.UUID) []byte {
	return []byte("proj_id:" + publicID.String())
}
