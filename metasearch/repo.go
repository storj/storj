package metasearch

import (
	"context"
	"encoding/json"
	"fmt"

	"storj.io/storj/satellite/metabase"
)

// MetaSearchRepo performs operations on object metadata.
type MetaSearchRepo interface {
	GetMetadata(ctx context.Context, loc metabase.ObjectLocation) (meta map[string]interface{}, err error)
	QueryMetadata(ctx context.Context, loc metabase.ObjectLocation, containsQuery map[string]interface{}, batchSize int) (metabase.FindObjectsByClearMetadataResult, error)
	UpdateMetadata(ctx context.Context, loc metabase.ObjectLocation, meta map[string]interface{}) (err error)
	DeleteMetadata(ctx context.Context, loc metabase.ObjectLocation) (err error)
}

type MetabaseSearchRepository struct {
	db *metabase.DB
}

// NewMetabaseSearchRepository creates a new MetabaseSearchRepository.
func NewMetabaseSearchRepository(db *metabase.DB) *MetabaseSearchRepository {
	return &MetabaseSearchRepository{
		db: db,
	}
}

func (r *MetabaseSearchRepository) GetMetadata(ctx context.Context, loc metabase.ObjectLocation) (meta map[string]interface{}, err error) {
	obj, err := r.db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{loc})
	if err != nil && metabase.ErrObjectNotFound.Has(err) {
		return nil, fmt.Errorf("%w: object not found", ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	if obj.ClearMetadata != nil {
		return parseJSON(*obj.ClearMetadata)
	}
	return nil, nil
}

func (r *MetabaseSearchRepository) UpdateMetadata(ctx context.Context, loc metabase.ObjectLocation, meta map[string]interface{}) (err error) {
	// Parse JSON metadata
	var newMetadata *string
	if meta != nil {
		data, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		s := string(data)
		newMetadata = &s
	}

	// Get current version
	current, err := r.db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{loc})
	if err != nil && metabase.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("%w: object not found", ErrNotFound)
	} else if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	// Update metadata
	obj := metabase.UpdateObjectLastCommittedMetadata{
		ObjectLocation: loc,
		ClearMetadata:  newMetadata,
		StreamID:       current.StreamID,
	}
	err = r.db.UpdateObjectLastCommittedMetadata(ctx, obj)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return nil
}

func (r *MetabaseSearchRepository) DeleteMetadata(ctx context.Context, loc metabase.ObjectLocation) (err error) {
	return r.UpdateMetadata(ctx, loc, nil)
}

func (r *MetabaseSearchRepository) QueryMetadata(ctx context.Context, loc metabase.ObjectLocation, containsQuery map[string]interface{}, batchSize int) (metabase.FindObjectsByClearMetadataResult, error) {
	query, err := json.Marshal(containsQuery)
	if err != nil {
		return metabase.FindObjectsByClearMetadataResult{}, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	opts := metabase.FindObjectsByClearMetadata{
		ProjectID:     loc.ProjectID,
		BucketName:    loc.BucketName,
		ContainsQuery: string(query),
	}

	startAfter := metabase.ObjectStream{
		ProjectID:  loc.ProjectID,
		BucketName: loc.BucketName,
		ObjectKey:  loc.ObjectKey,
	}

	return r.db.FindObjectsByClearMetadata(ctx, opts, startAfter, batchSize)
}

func parseJSON(data string) (map[string]interface{}, error) {
	var meta map[string]interface{}
	err := json.Unmarshal([]byte(data), &meta)
	if err != nil {
		return nil, err
	}
	return meta, nil
}
