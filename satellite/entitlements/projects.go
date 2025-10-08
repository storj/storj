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

// ProjectScopePrefix is the prefix used for project scopes in the database.
const ProjectScopePrefix = "proj_id:"

// PlacementProductMappings maps placements to their corresponding product IDs.
type PlacementProductMappings map[storj.PlacementConstraint]int32

// ProjectFeatures defines the features available for a project.
type ProjectFeatures struct {
	NewBucketPlacements      []storj.PlacementConstraint `json:"new_bucket_placements,omitempty"`
	PlacementProductMappings PlacementProductMappings    `json:"placement_product_mappings,omitempty"`
	ComputeAccessToken       []byte                      `json:"compute_access_token,omitempty"`
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

// SetPlacementProductMappingsByPublicID sets the placement product mappings for a project by its public ID.
func (p *Projects) SetPlacementProductMappingsByPublicID(ctx context.Context, publicID uuid.UUID, newMappings PlacementProductMappings) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newMappings == nil {
		return Error.New("placement:product mappings cannot be empty")
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
	features.PlacementProductMappings = newMappings

	return Error.Wrap(p.upsertNewEntitlement(ctx, ent, features))
}

// SetComputeAccessTokenByPublicID sets the compute access token for a project by its public ID.
func (p *Projects) SetComputeAccessTokenByPublicID(ctx context.Context, publicProjectID uuid.UUID, token []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	scope := ConvertPublicIDToProjectScope(publicProjectID)

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
	features.ComputeAccessToken = token

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
	return append([]byte(ProjectScopePrefix), publicID[:]...)
}
