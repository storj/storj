// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/metainfo/metaloop"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/storage"
	"storj.io/storj/storage/cockroachkv"
	"storj.io/storj/storage/postgreskv"
)

const (
	// BoltPointerBucket is the string representing the bucket used for `PointerEntries` in BoltDB.
	BoltPointerBucket = "pointers"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information.
//
// Can be used as a flag.
type RSConfig struct {
	ErasureShareSize memory.Size
	Min              int
	Repair           int
	Success          int
	Total            int
}

// Type implements pflag.Value.
func (RSConfig) Type() string { return "metainfo.RSConfig" }

// String is required for pflag.Value.
func (rs *RSConfig) String() string {
	return fmt.Sprintf("%d/%d/%d/%d-%s",
		rs.Min,
		rs.Repair,
		rs.Success,
		rs.Total,
		rs.ErasureShareSize.String())
}

// Set sets the value from a string in the format k/m/o/n-size (min/repair/optimal/total-erasuresharesize).
func (rs *RSConfig) Set(s string) error {
	// Split on dash. Expect two items. First item is RS numbers. Second item is memory.Size.
	info := strings.Split(s, "-")
	if len(info) != 2 {
		return Error.New("Invalid default RS config (expect format k/m/o/n-ShareSize, got %s)", s)
	}
	rsNumbersString := info[0]
	shareSizeString := info[1]

	// Attempt to parse "-size" part of config.
	shareSizeInt, err := memory.ParseString(shareSizeString)
	if err != nil {
		return Error.New("Invalid share size in RS config: '%s', %w", shareSizeString, err)
	}
	shareSize := memory.Size(shareSizeInt)

	// Split on forward slash. Expect exactly four positive non-decreasing integers.
	rsNumbers := strings.Split(rsNumbersString, "/")
	if len(rsNumbers) != 4 {
		return Error.New("Invalid default RS numbers (wrong size, expect 4): %s", rsNumbersString)
	}

	minValue := 1
	values := []int{}
	for _, nextValueString := range rsNumbers {
		nextValue, err := strconv.Atoi(nextValueString)
		if err != nil {
			return Error.New("Invalid default RS numbers (should all be valid integers): %s, %w", rsNumbersString, err)
		}
		if nextValue < minValue {
			return Error.New("Invalid default RS numbers (should be non-decreasing): %s", rsNumbersString)
		}
		values = append(values, nextValue)
		minValue = nextValue
	}

	rs.ErasureShareSize = shareSize
	rs.Min = values[0]
	rs.Repair = values[1]
	rs.Success = values[2]
	rs.Total = values[3]

	return nil
}

// RateLimiterConfig is a configuration struct for endpoint rate limiting.
type RateLimiterConfig struct {
	Enabled         bool          `help:"whether rate limiting is enabled." releaseDefault:"true" devDefault:"true"`
	Rate            float64       `help:"request rate per project per second." releaseDefault:"1000" devDefault:"100"`
	CacheCapacity   int           `help:"number of projects to cache." releaseDefault:"10000" devDefault:"10"`
	CacheExpiration time.Duration `help:"how long to cache the projects limiter." releaseDefault:"10m" devDefault:"10s"`
}

// ProjectLimitConfig is a configuration struct for default project limits.
type ProjectLimitConfig struct {
	MaxBuckets int `help:"max bucket count for a project." default:"100"`
}

// Config is a configuration struct that is everything you need to start a metainfo.
type Config struct {
	DatabaseURL          string               `help:"the database connection string to use" default:"postgres://"`
	MinRemoteSegmentSize memory.Size          `default:"1240" help:"minimum remote segment size"`
	MaxInlineSegmentSize memory.Size          `default:"4KiB" help:"maximum inline segment size"`
	MaxSegmentSize       memory.Size          `default:"64MiB" help:"maximum segment size"`
	MaxMetadataSize      memory.Size          `default:"2KiB" help:"maximum segment metadata size"`
	MaxCommitInterval    time.Duration        `default:"48h" help:"maximum time allowed to pass between creating and committing a segment"`
	Overlay              bool                 `default:"true" help:"toggle flag if overlay is enabled"`
	RS                   RSConfig             `releaseDefault:"29/35/80/110-256B" devDefault:"4/6/8/10-256B" help:"redundancy scheme configuration in the format k/m/o/n-sharesize"`
	Loop                 metaloop.Config      `help:"loop configuration"`
	RateLimiter          RateLimiterConfig    `help:"rate limiter configuration"`
	ProjectLimits        ProjectLimitConfig   `help:"project limit configuration"`
	PieceDeletion        piecedeletion.Config `help:"piece deletion configuration"`
}

// PointerDB stores pointers.
//
// architecture: Database
type PointerDB interface {
	// MigrateToLatest migrates to latest schema version.
	MigrateToLatest(ctx context.Context) error

	storage.KeyValueStore
}

// OpenStore returns database for storing pointer data.
func OpenStore(ctx context.Context, logger *zap.Logger, dbURLString string, app string) (db PointerDB, err error) {
	_, source, implementation, err := dbutil.SplitConnStr(dbURLString)
	if err != nil {
		return nil, err
	}

	switch implementation {
	case dbutil.Postgres:
		db, err = postgreskv.Open(ctx, source, app)
	case dbutil.Cockroach:
		db, err = cockroachkv.Open(ctx, source, app)
	default:
		err = Error.New("unsupported db implementation: %s", dbURLString)
	}

	if err != nil {
		return nil, err
	}

	logger.Debug("Connected to:", zap.String("db source", source))
	return db, nil
}

// MetabaseDB stores objects and segments.
type MetabaseDB interface {
	io.Closer

	// MigrateToLatest migrates to latest schema version.
	MigrateToLatest(ctx context.Context) error
	// CheckVersion checks the database is the correct version
	CheckVersion(ctx context.Context) error

	// DeleteObjectAnyStatusAllVersions deletes all object versions.
	DeleteObjectAnyStatusAllVersions(ctx context.Context, opts metabase.DeleteObjectAnyStatusAllVersions) (result metabase.DeleteObjectResult, err error)
	// DeleteObjectsAllVersions deletes all versions of multiple objects from the same bucket.
	DeleteObjectsAllVersions(ctx context.Context, opts metabase.DeleteObjectsAllVersions) (result metabase.DeleteObjectResult, err error)
	// DeletePendingObject deletes a pending object.
	DeletePendingObject(ctx context.Context, opts metabase.DeletePendingObject) (result metabase.DeleteObjectResult, err error)
	// DeleteBucketObjects deletes all objects in the specified bucket.
	DeleteBucketObjects(ctx context.Context, opts metabase.DeleteBucketObjects) (deletedObjectCount int64, err error)
	// DeleteExpiredObjects deletes all objects that expired before expiredBefore.
	DeleteExpiredObjects(ctx context.Context, opts metabase.DeleteExpiredObjects) error
	// DeleteObjectLatestVersion deletes latest object version.
	DeleteObjectLatestVersion(ctx context.Context, opts metabase.DeleteObjectLatestVersion) (result metabase.DeleteObjectResult, err error)
	// BeginObjectExactVersion adds a pending object to the database, with specific version.
	BeginObjectExactVersion(ctx context.Context, opts metabase.BeginObjectExactVersion) (committed metabase.Object, err error)
	// CommitObject adds a pending object to the database.
	CommitObject(ctx context.Context, opts metabase.CommitObject) (object metabase.Object, err error)
	// BeginSegment verifies whether a new segment upload can be started.
	BeginSegment(ctx context.Context, opts metabase.BeginSegment) (err error)
	// CommitSegment commits segment to the database.
	CommitSegment(ctx context.Context, opts metabase.CommitSegment) (err error)
	// CommitInlineSegment commits inline segment to the database.
	CommitInlineSegment(ctx context.Context, opts metabase.CommitInlineSegment) (err error)
	// GetObjectLatestVersion returns object information for latest version.
	GetObjectLatestVersion(ctx context.Context, opts metabase.GetObjectLatestVersion) (_ metabase.Object, err error)
	// GetSegmentByLocation returns a information about segment on the specified location.
	GetSegmentByLocation(ctx context.Context, opts metabase.GetSegmentByLocation) (segment metabase.Segment, err error)
	// GetSegmentByPosition returns a information about segment which covers specified offset.
	GetSegmentByPosition(ctx context.Context, opts metabase.GetSegmentByPosition) (segment metabase.Segment, err error)
	// GetLatestObjectLastSegment returns an object last segment information.
	GetLatestObjectLastSegment(ctx context.Context, opts metabase.GetLatestObjectLastSegment) (segment metabase.Segment, err error)
	// GetStreamPieceCountByNodeID returns piece count by node id.
	GetStreamPieceCountByNodeID(ctx context.Context, opts metabase.GetStreamPieceCountByNodeID) (result map[storj.NodeID]int64, err error)
	// ListSegments lists specified stream segments.
	ListSegments(ctx context.Context, opts metabase.ListSegments) (result metabase.ListSegmentsResult, err error)
	// ListStreamPositions lists specified stream segment positions.
	ListStreamPositions(ctx context.Context, opts metabase.ListStreamPositions) (result metabase.ListStreamPositionsResult, err error)
	// IterateObjectsAllVersions iterates through all versions of all objects.
	IterateObjectsAllVersions(ctx context.Context, opts metabase.IterateObjects, fn func(context.Context, metabase.ObjectsIterator) error) (err error)
	// IterateObjectsAllVersionsWithStatus iterates through all versions of all objects with specified status.
	IterateObjectsAllVersionsWithStatus(ctx context.Context, opts metabase.IterateObjectsWithStatus, fn func(context.Context, metabase.ObjectsIterator) error) (err error)
	// IteratePendingObjectsByKey iterates through all StreamID for a given ObjectKey.
	IteratePendingObjectsByKey(ctx context.Context, opts metabase.IteratePendingObjectsByKey, fn func(context.Context, metabase.ObjectsIterator) error) (err error)
	// IterateLoopObjects iterates through all objects in metabase for metainfo loop purpose.
	IterateLoopObjects(ctx context.Context, opts metabase.IterateLoopObjects, fn func(context.Context, metabase.LoopObjectsIterator) error) (err error)
	// IterateLoopStreams iterates through all streams passed in as arguments.
	IterateLoopStreams(ctx context.Context, opts metabase.IterateLoopStreams, handleStream func(ctx context.Context, streamID uuid.UUID, next metabase.SegmentIterator) error) (err error)
	// BucketEmpty returns true if bucket does not contain objects (pending or committed).
	// This method doesn't check bucket existence.
	BucketEmpty(ctx context.Context, opts metabase.BucketEmpty) (empty bool, err error)
	// UpdateSegmentPieces updates pieces for specified segment. If provided old pieces won't match current database state update will fail.
	UpdateSegmentPieces(ctx context.Context, opts metabase.UpdateSegmentPieces) (err error)
	// EnsureNodeAliases ensures that the supplied node ID-s have a alias.
	// It's safe to concurrently try and create node ID-s for the same NodeID.
	EnsureNodeAliases(ctx context.Context, opts metabase.EnsureNodeAliases) (err error)
	// ListNodeAliases lists all node alias mappings.
	ListNodeAliases(ctx context.Context) (_ []metabase.NodeAliasEntry, err error)

	// TestingAllCommittedObjects gets all committed objects from bucket. Use only for testing purposes.
	TestingAllCommittedObjects(ctx context.Context, projectID uuid.UUID, bucketName string) (objects []metabase.ObjectEntry, err error)
	// TestingAllPendingObjects gets all pending objects from bucket. Use only for testing purposes.
	TestingAllPendingObjects(ctx context.Context, projectID uuid.UUID, bucketName string) (objects []metabase.ObjectEntry, err error)
	// TestingAllObjectSegments gets all segments for given object. Use only for testing purposes.
	TestingAllObjectSegments(ctx context.Context, objectLocation metabase.ObjectLocation) (segments []metabase.Segment, err error)
	// TestingAllObjects gets all objects. Use only for testing purposes.
	TestingAllObjects(ctx context.Context) (segments []metabase.Object, err error)
	// TestingAllSegments gets all segments. Use only for testing purposes.
	TestingAllSegments(ctx context.Context) (segments []metabase.Segment, err error)

	// InternalImplementation returns *metabase.DB.
	// TODO: remove.
	InternalImplementation() interface{}
}

// OpenMetabase returns database for storing objects and segments.
func OpenMetabase(ctx context.Context, log *zap.Logger, dbURLString string) (db MetabaseDB, err error) {
	_, source, implementation, err := dbutil.SplitConnStr(dbURLString)
	if err != nil {
		return nil, err
	}

	switch implementation {
	case dbutil.Postgres:
		db, err = metabase.Open(ctx, log, "pgx", dbURLString)
	case dbutil.Cockroach:
		db, err = metabase.Open(ctx, log, "cockroach", dbURLString)
	default:
		err = Error.New("unsupported db implementation: %s", dbURLString)
	}

	if err != nil {
		return nil, err
	}

	log.Debug("Connected to:", zap.String("db source", source))
	return db, nil
}
