// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"
	"fmt"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var (
	// ErrBucket is an error class for general bucket errors.
	ErrBucket = errs.Class("bucket")

	// ErrNoBucket is an error class for using empty bucket name.
	ErrNoBucket = errs.Class("no bucket specified")

	// ErrBucketNotFound is an error class for non-existing bucket.
	ErrBucketNotFound = errs.Class("bucket not found")

	// ErrBucketAlreadyExists is used to indicate that bucket already exists.
	ErrBucketAlreadyExists = errs.Class("bucket already exists")

	// ErrConflict is used when a request conflicts with the state of a bucket.
	ErrConflict = errs.Class("bucket operation conflict")

	// ErrUnavailable is used when an operation is temporarily unavailable
	// due to a transient issue with the database's state.
	ErrUnavailable = errs.Class("bucket operation temporarily unavailable")

	// ErrLocked is used when an operation fails because a bucket has Object Lock enabled.
	ErrLocked = errs.Class("bucket has Object Lock enabled")
)

// Bucket contains information about a specific bucket.
type Bucket struct {
	ID         uuid.UUID
	Name       string
	ProjectID  uuid.UUID
	CreatedBy  uuid.UUID
	UserAgent  []byte
	Created    time.Time
	Placement  storj.PlacementConstraint
	Versioning Versioning
	ObjectLock ObjectLockSettings
}

// UpdateBucketObjectLockParams contains the parameters for updating bucket object lock settings.
type UpdateBucketObjectLockParams struct {
	ProjectID             uuid.UUID
	Name                  string
	ObjectLockEnabled     bool
	DefaultRetentionMode  **storj.RetentionMode
	DefaultRetentionDays  **int
	DefaultRetentionYears **int
}

// ObjectLockSettings contains a bucket's object lock configurations.
type ObjectLockSettings struct {
	Enabled               bool
	DefaultRetentionMode  storj.RetentionMode
	DefaultRetentionDays  int
	DefaultRetentionYears int
}

// ListDirection specifies listing direction.
type ListDirection = pb.ListDirection

const (
	// DirectionForward lists forwards from cursor, including cursor.
	DirectionForward = pb.ListDirection_FORWARD
	// DirectionAfter lists forwards from cursor, without cursor.
	DirectionAfter = pb.ListDirection_AFTER
)

// Versioning represents the versioning state of a bucket.
type Versioning int

const (
	// VersioningUnsupported represents a bucket where versioning is not supported.
	VersioningUnsupported Versioning = 0
	// Unversioned represents a bucket where versioning has never been enabled.
	Unversioned Versioning = 1
	// VersioningEnabled represents a bucket where versioning is enabled.
	VersioningEnabled Versioning = 2
	// VersioningSuspended represents a bucket where versioning is currently suspended.
	VersioningSuspended Versioning = 3
)

// IsUnversioned returns true if bucket state represents unversioned bucket.
func (v Versioning) IsUnversioned() bool {
	return v == VersioningUnsupported || v == Unversioned
}

// IsVersioned returns true if bucket is either in a versioned or suspended state.
func (v Versioning) IsVersioned() bool {
	return !v.IsUnversioned()
}

// String returns the name.
func (v Versioning) String() string {
	switch v {
	case VersioningUnsupported:
		return "unsupported"
	case Unversioned:
		return "unversioned"
	case VersioningEnabled:
		return "enabled"
	case VersioningSuspended:
		return "suspended"
	default:
		return fmt.Sprintf("unknown Versioning(%d)", v)
	}
}

// Tag represents a single bucket tag.
type Tag struct {
	Key   string
	Value string
}

// MinimalBucket contains minimal bucket fields for metainfo protocol.
type MinimalBucket struct {
	Name      []byte
	CreatedBy uuid.UUID
	CreatedAt time.Time
	Placement storj.PlacementConstraint
}

// NotificationConfig contains bucket event notification configuration.
type NotificationConfig struct {
	ConfigID     string
	TopicName    string
	Events       []string
	FilterPrefix []byte
	FilterSuffix []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListOptions lists objects.
type ListOptions struct {
	Cursor    string
	Direction ListDirection
	Limit     int
}

// NextPage returns options for listing the next page.
func (opts ListOptions) NextPage(list List) ListOptions {
	if !list.More || len(list.Items) == 0 {
		return ListOptions{}
	}

	return ListOptions{
		Cursor:    list.Items[len(list.Items)-1].Name,
		Direction: DirectionAfter,
		Limit:     opts.Limit,
	}
}

// List is a list of buckets.
type List struct {
	More  bool
	Items []Bucket
}

// DB is the interface for the database to interact with buckets.
//
// architecture: Database
type DB interface {
	// CreateBucket creates a new bucket
	CreateBucket(ctx context.Context, bucket Bucket) (_ Bucket, err error)
	// GetBucket returns an existing bucket
	GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket Bucket, err error)
	// GetBucketPlacement returns with the placement constraint identifier.
	GetBucketPlacement(ctx context.Context, bucketName []byte, projectID uuid.UUID) (placement storj.PlacementConstraint, err error)
	// GetBucketVersioningState returns with the versioning state of the bucket.
	GetBucketVersioningState(ctx context.Context, bucketName []byte, projectID uuid.UUID) (versioningState Versioning, err error)
	// EnableBucketVersioning enables versioning for a bucket.
	EnableBucketVersioning(ctx context.Context, bucketName []byte, projectID uuid.UUID) error
	// SuspendBucketVersioning suspends versioning for a bucket.
	SuspendBucketVersioning(ctx context.Context, bucketName []byte, projectID uuid.UUID) error
	// GetMinimalBucket returns existing bucket with minimal number of fields.
	GetMinimalBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket MinimalBucket, err error)
	// HasBucket returns if a bucket exists.
	HasBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (exists bool, err error)
	// UpdateBucket updates an existing bucket
	UpdateBucket(ctx context.Context, bucket Bucket) (_ Bucket, err error)
	// UpdateUserAgent updates buckets user agent.
	UpdateUserAgent(ctx context.Context, projectID uuid.UUID, bucketName string, userAgent []byte) error
	// UpdateBucketObjectLockSettings updates object lock settings for a bucket without an extra database query.
	UpdateBucketObjectLockSettings(ctx context.Context, params UpdateBucketObjectLockParams) (_ Bucket, err error)
	// GetBucketObjectLockSettings returns a bucket's object lock settings.
	GetBucketObjectLockSettings(ctx context.Context, bucketName []byte, projectID uuid.UUID) (settings *ObjectLockSettings, err error)
	// DeleteBucket deletes a bucket
	DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error)
	// ListBuckets returns all buckets for a project
	ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts ListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList List, err error)
	// CountBuckets returns the number of buckets a project currently has
	CountBuckets(ctx context.Context, projectID uuid.UUID) (int, error)
	// CountObjectLockBuckets returns the number of buckets a project currently has with object lock enabled.
	CountObjectLockBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error)
	// IterateBucketLocations iterates through all buckets with specific page size.
	IterateBucketLocations(ctx context.Context, pageSize int, fn func([]metabase.BucketLocation) error) (err error)
	// GetBucketObjectLockEnabled returns whether a bucket has Object Lock enabled.
	GetBucketObjectLockEnabled(ctx context.Context, bucketName []byte, projectID uuid.UUID) (enabled bool, err error)
	// GetBucketTagging returns the set of tags placed on a bucket.
	GetBucketTagging(ctx context.Context, bucketName []byte, projectID uuid.UUID) (tags []Tag, err error)
	// SetBucketTagging places a set of tags on a bucket.
	SetBucketTagging(ctx context.Context, bucketName []byte, projectID uuid.UUID, tags []Tag) (err error)
	// UpdateBucketNotificationConfig updates the bucket notification configuration for a bucket.
	UpdateBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID, config NotificationConfig) error
	// GetBucketNotificationConfig retrieves the notification configuration for a bucket.
	// Returns nil if no configuration exists.
	GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (*NotificationConfig, error)
	// DeleteBucketNotificationConfig removes the notification configuration for a bucket.
	DeleteBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) error
}
