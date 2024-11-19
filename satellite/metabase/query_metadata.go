package metabase

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
)

type FindObjectsByClearMetadata struct {
	ProjectID     uuid.UUID
	BucketName    BucketName
	ContainsQuery string
}

type FindObjectsByClearMetadataResult struct {
	Objects []FindObjectsByClearMetadataResultObject
}

type FindObjectsByClearMetadataResultObject struct {
	ObjectStream
	ClearMetadata string
}

func (db *DB) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)
	return db.ChooseAdapter(opts.ProjectID).FindObjectsByClearMetadata(ctx, opts, startAfter, batchSize)
}

func (p *PostgresAdapter) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT
			project_id, bucket_name, object_key, version, stream_id, clear_metadata
		FROM objects
		WHERE
			(project_id, bucket_name) = ($1, $2) AND
			(project_id, bucket_name, object_key, version) > ($3, $4, $5, $6) AND
			clear_metadata @> $7 AND
			status <> ` + statusPending + ` AND
			(expires_at IS NULL OR expires_at > now())
		ORDER BY project_id, bucket_name, object_key, version
		LIMIT $8;
	`

	result.Objects = make([]FindObjectsByClearMetadataResultObject, 0, batchSize)

	err = withRows(p.db.QueryContext(ctx, query,
		opts.ProjectID, opts.BucketName,
		startAfter.ProjectID, startAfter.BucketName, []byte(startAfter.ObjectKey), startAfter.Version,
		opts.ContainsQuery,
		batchSize),
	)(func(rows tagsql.Rows) error {
		var last FindObjectsByClearMetadataResultObject
		for rows.Next() {
			err = rows.Scan(
				&last.ProjectID, &last.BucketName, &last.ObjectKey, &last.Version, &last.StreamID, &last.ClearMetadata)
			if err != nil {
				return Error.Wrap(err)
			}

			p.log.Debug("Querying objects by clear metadata",
				zap.Stringer("Project", last.ProjectID),
				zap.Stringer("Bucket", last.BucketName),
				zap.String("Object Key", string(last.ObjectKey)),
				zap.Int64("Version", int64(last.Version)),
				zap.Stringer("StreamID", last.StreamID),
			)
			result.Objects = append(result.Objects, last)
		}

		return nil
	})
	if err != nil {
		return FindObjectsByClearMetadataResult{}, Error.Wrap(err)
	}
	return result, nil
}

func (p *SpannerAdapter) FindObjectsByClearMetadata(ctx context.Context, opts FindObjectsByClearMetadata, startAfter ObjectStream, batchSize int) (result FindObjectsByClearMetadataResult, err error) {
	return FindObjectsByClearMetadataResult{}, errors.New("not implemented")
}
