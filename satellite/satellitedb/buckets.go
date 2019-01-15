package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/consoledbx"
)

type buckets struct {
	db dbx.Methods
}

func (buck *buckets) ListBuckets(ctx context.Context, projectID uuid.UUID) ([]console.Bucket, error) {
	buckets, err := buck.db.All_Bucket_By_ProjectId_OrderBy_Asc_Name(
		ctx,
		dbx.Bucket_ProjectId(projectID[:]),
	)

	if err != nil {
		return nil, err
	}

	var consoleBuckets []console.Bucket
	for _, bucket := range buckets {
		consoleBucket, bucketErr := fromDBXBucket(bucket)
		if err != nil {
			err = errs.Combine(err, bucketErr)
			continue
		}

		consoleBuckets = append(consoleBuckets, *consoleBucket)
	}

	if err != nil {
		return nil, err
	}

	return consoleBuckets, nil
}

func (buck *buckets) GetBucket(ctx context.Context, name string) (*console.Bucket, error) {
	bucket, err := buck.db.Get_Bucket_By_Name(ctx, dbx.Bucket_Name(name))
	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

func (buck *buckets) AttachBucket(ctx context.Context, name string, projectID uuid.UUID) (*console.Bucket, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	bucket, err := buck.db.Create_Bucket(
		ctx,
		dbx.Bucket_Id(id[:]),
		dbx.Bucket_ProjectId(projectID[:]),
		dbx.Bucket_Name(name),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

func (buck *buckets) DeattachBucket(ctx context.Context, name string) error {
	panic("implement me")
}

func fromDBXBucket(bucket *dbx.Bucket) (*console.Bucket, error) {
	id, err := bytesToUUID(bucket.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(bucket.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.Bucket{
		ID:        id,
		ProjectID: projectID,
		Name:      bucket.Name,
		CreatedAt: bucket.CreatedAt,
	}, nil
}
