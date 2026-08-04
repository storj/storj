// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"storj.io/storj/shared/tagsql"
)

// BucketTally contains information about aggregate data stored in a bucket.
type BucketTally struct {
	BucketLocation

	ObjectCount        int64
	PendingObjectCount int64

	TotalSegments int64
	TotalBytes    int64

	MetadataSize int64

	// BytesByRemainder maps storage remainder values to total bytes calculated with that remainder.
	// The map key is the remainder value in bytes, and the value is the total bytes for this bucket
	// calculated with that remainder applied.
	BytesByRemainder map[int64]int64
}

// CollectBucketTallies contains arguments necessary for looping through objects in metabase.
type CollectBucketTallies struct {
	From               BucketLocation
	To                 BucketLocation
	AsOfSystemTime     time.Time
	AsOfSystemInterval time.Duration
	Now                time.Time

	UsePartitionQuery bool

	// StorageRemainders is a list of remainder values to calculate for each bucket.
	// Objects with total_encrypted_size less than a remainder value are counted as that remainder value.
	// Results are returned in BucketTally.BytesByRemainder map.
	//
	// Example: []int64{0, 51200, 102400} will calculate three values per bucket:
	//   - BytesByRemainder[0] = actual total size
	//   - BytesByRemainder[51200] = total size with 50KB minimum per object
	//   - BytesByRemainder[102400] = total size with 100KB minimum per object
	//
	// An empty list defaults to []int64{0} (no remainder applied).
	StorageRemainders []int64
}

// Verify verifies CollectBucketTallies request fields.
func (opts *CollectBucketTallies) Verify() error {
	if opts.To.ProjectID.Less(opts.From.ProjectID) {
		return ErrInvalidRequest.New("project ID To is before project ID From")
	}
	if opts.To.ProjectID == opts.From.ProjectID && opts.To.BucketName < opts.From.BucketName {
		return ErrInvalidRequest.New("bucket name To is before bucket name From")
	}
	return nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (db *DB) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return []BucketTally{}, err
	}

	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	for _, adapter := range db.adapters {
		adapterResult, err := adapter.CollectBucketTallies(ctx, opts)
		if err != nil {
			return nil, err
		}
		result = append(result, adapterResult...)
	}

	// only a merge sort should be strictly required here, but this is much easier to implement for now
	slices.SortFunc(result, func(a, b BucketTally) int {
		return a.BucketLocation.Compare(b.BucketLocation)
	})

	return result, nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (p *PostgresAdapter) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	remainders := normalizeStorageRemainders(opts.StorageRemainders)

	// Build dynamic SUM columns for each remainder value.
	var sumExpressions []string
	paramIdx := 6 // Start after the 5 base parameters.
	for _, remainder := range remainders {
		if remainder > 0 {
			sumExpressions = append(sumExpressions, fmt.Sprintf("SUM(GREATEST(total_encrypted_size, $%d))", paramIdx))
			paramIdx++
		} else {
			sumExpressions = append(sumExpressions, "SUM(total_encrypted_size)")
		}
	}

	// Build the query with multiple remainder calculations.
	selectCols := "project_id, bucket_name, " + strings.Join(sumExpressions, ", ") + `,
		SUM(segment_count),
		COALESCE(SUM(length(encrypted_metadata)), 0) + COALESCE(SUM(length(encrypted_etag)), 0),
		count(*),
		count(*) FILTER (WHERE status = ` + statusPending + `)`

	query := `
		SELECT
			` + selectCols + `
		FROM objects
		` + LimitedAsOfSystemTime(p.impl, time.Now(), opts.AsOfSystemTime, opts.AsOfSystemInterval) + `
		WHERE (project_id, bucket_name) BETWEEN ($1, $2) AND ($3, $4) AND
		(expires_at IS NULL OR expires_at > $5)
		GROUP BY (project_id, bucket_name)
		ORDER BY (project_id, bucket_name) ASC
	`

	// Build query arguments.
	args := []any{opts.From.ProjectID, opts.From.BucketName, opts.To.ProjectID, opts.To.BucketName, opts.Now}
	for _, remainder := range remainders {
		if remainder > 0 {
			args = append(args, remainder)
		}
	}

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return []BucketTally{}, Error.New("unable to query bucket tallies: %w", err)
	}

	err = withRows(rows, err)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var bucketTally BucketTally

			// Prepare slice to scan remainder bytes.
			bytesByRemainder := make([]int64, len(remainders))
			scanDest := []any{
				&bucketTally.ProjectID, &bucketTally.BucketName,
			}
			for i := range remainders {
				scanDest = append(scanDest, &bytesByRemainder[i])
			}
			scanDest = append(scanDest,
				&bucketTally.TotalSegments,
				&bucketTally.MetadataSize,
				&bucketTally.ObjectCount,
				&bucketTally.PendingObjectCount,
			)

			if err = rows.Scan(scanDest...); err != nil {
				return Error.New("unable to query bucket tally: %w", err)
			}

			// Populate BytesByRemainder map with all calculated values.
			bucketTally.BytesByRemainder = make(map[int64]int64)
			for i, remainder := range remainders {
				bucketTally.BytesByRemainder[remainder] = bytesByRemainder[i]
			}

			// For backward compatibility, populate TotalBytes with actual bytes (remainder=0).
			// We always ensure remainder=0 is in the list, so BytesByRemainder[0] is always present.
			bucketTally.TotalBytes = bucketTally.BytesByRemainder[0]

			result = append(result, bucketTally)
		}

		return nil
	})
	if err != nil {
		return []BucketTally{}, err
	}

	return result, nil
}

// CollectBucketTallies collect limited bucket tallies from given bucket locations.
func (t *TiDBAdapter) CollectBucketTallies(ctx context.Context, opts CollectBucketTallies) (result []BucketTally, err error) {
	defer mon.Task()(&ctx)(&err)

	remainders := normalizeStorageRemainders(opts.StorageRemainders)

	// TiKV cannot evaluate GREATEST and TiDB's aggregation pushdown is
	// all-or-nothing, so GREATEST in the SELECT would force the whole
	// GROUP BY to run on the TiDB server. CASE WHEN pushes down.
	var sumExpressions []string
	for _, remainder := range remainders {
		if remainder > 0 {
			sumExpressions = append(sumExpressions, "SUM(CASE WHEN total_encrypted_size < ? THEN ? ELSE total_encrypted_size END)")
		} else {
			sumExpressions = append(sumExpressions, "SUM(total_encrypted_size)")
		}
	}

	selectCols := "project_id, bucket_name, " + strings.Join(sumExpressions, ", ") + `,
		SUM(segment_count),
		COALESCE(SUM(LENGTH(encrypted_metadata)), 0) + COALESCE(SUM(LENGTH(encrypted_etag)), 0),
		COUNT(*),
		SUM(CASE WHEN status = ` + statusPending + ` THEN 1 ELSE 0 END)`

	asOf := LimitedAsOfSystemTimeBounded(t.Implementation(), time.Now(), opts.AsOfSystemTime, opts.AsOfSystemInterval)

	// TiDB derives scan ranges only from the project_id parts of the OR-form
	// tuple comparison, leaving bucket_name bounds as per-row filters that
	// re-scan every bucket of the boundary projects. Split the range into
	// pieces whose bounds enter the scan range directly. Group keys are
	// disjoint across pieces, so UNION ALL needs no re-aggregation.
	var pieces []string
	var args []any
	addPiece := func(rangeCond string, rangeArgs ...any) {
		pieces = append(pieces, `(
		SELECT
			`+selectCols+`
		FROM objects
		`+asOf+`
		WHERE `+rangeCond+`
			AND (expires_at IS NULL OR expires_at > ?)
		GROUP BY project_id, bucket_name
		)`)
		for _, remainder := range remainders {
			if remainder > 0 {
				args = append(args, remainder, remainder)
			}
		}
		args = append(args, rangeArgs...)
		args = append(args, opts.Now)
	}

	if opts.From.ProjectID == opts.To.ProjectID {
		addPiece("project_id = ? AND bucket_name >= ? AND bucket_name <= ?",
			opts.From.ProjectID, opts.From.BucketName, opts.To.BucketName)
	} else {
		addPiece("project_id = ? AND bucket_name >= ?", opts.From.ProjectID, opts.From.BucketName)
		addPiece("project_id > ? AND project_id < ?", opts.From.ProjectID, opts.To.ProjectID)
		addPiece("project_id = ? AND bucket_name <= ?", opts.To.ProjectID, opts.To.BucketName)
	}

	query := strings.Join(pieces, " UNION ALL ") + " ORDER BY project_id ASC, bucket_name ASC"

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		return []BucketTally{}, Error.New("unable to query bucket tallies: %w", err)
	}

	err = withRows(rows, err)(func(rows tagsql.Rows) error {
		for rows.Next() {
			var bucketTally BucketTally
			bytesByRemainder := make([]int64, len(remainders))
			scanDest := []any{
				&bucketTally.ProjectID, &bucketTally.BucketName,
			}
			for i := range remainders {
				scanDest = append(scanDest, &bytesByRemainder[i])
			}
			scanDest = append(scanDest,
				&bucketTally.TotalSegments,
				&bucketTally.MetadataSize,
				&bucketTally.ObjectCount,
				&bucketTally.PendingObjectCount,
			)
			if err = rows.Scan(scanDest...); err != nil {
				return Error.New("unable to query bucket tally: %w", err)
			}
			bucketTally.BytesByRemainder = make(map[int64]int64)
			for i, remainder := range remainders {
				bucketTally.BytesByRemainder[remainder] = bytesByRemainder[i]
			}
			bucketTally.TotalBytes = bucketTally.BytesByRemainder[0]
			result = append(result, bucketTally)
		}
		return nil
	})
	if err != nil {
		return []BucketTally{}, err
	}
	return result, nil
}

// normalizeStorageRemainders ensures remainder=0 is always included for backward compatibility.
// Returns a new slice with remainder=0 prepended if not already present.
func normalizeStorageRemainders(remainders []int64) []int64 {
	if len(remainders) == 0 {
		return []int64{0}
	}

	// Check if 0 is already in the list.
	if slices.Contains(remainders, 0) {
		return remainders
	}

	// Prepend 0 to the list so TotalBytes gets actual bytes.
	return append([]int64{0}, remainders...)
}
