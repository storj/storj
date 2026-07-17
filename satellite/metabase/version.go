// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
)

const (
	// The generateNextVersion snippets clamp the result to >= 1 (matching
	// nextVersion in Go), because pending objects created by the alternative
	// begin-object implementation have large negative versions. Without the
	// clamp a key holding only such rows would yield a negative next version —
	// on TiDB the LAST_INSERT_ID wrapper converts it to unsigned and the
	// INSERT fails with "Out of range value for column 'version'".
	postgresGenerateNextVersion = `greatest(coalesce((
		SELECT version + 1
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
		ORDER BY version DESC
		LIMIT 1
	), 1), 1)`
	spannerGenerateNextVersion = `greatest(coalesce(
		(SELECT version + 1
		FROM objects
		WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
		ORDER BY version DESC
		LIMIT 1)
	,1), 1)`
	tidbGenerateNextVersion = `greatest(coalesce(
		(SELECT version + 1
		FROM objects
		WHERE (project_id, bucket_name, object_key) = (?, ?, ?)
		ORDER BY version DESC
		LIMIT 1)
	,1), 1)`
	// tidbGenerateNextVersionLastInsertID wraps tidbGenerateNextVersion in
	// LAST_INSERT_ID(expr) so the chosen value lands in the INSERT's OK-packet
	// last_insert_id field and can be read back via sql.Result.LastInsertId()
	// without a follow-up query. LAST_INSERT_ID yields BIGINT UNSIGNED, so
	// CAST back to SIGNED for the column; the driver's uint64→int64 conversion
	// of the OK packet restores negative values, keeping the round trip
	// value-preserving.
	tidbGenerateNextVersionLastInsertID = `CAST(LAST_INSERT_ID(` + tidbGenerateNextVersion + `) AS SIGNED)`
	// tidbGenerateTimestampVersion is a SQL snippet that yields the current time
	// in microseconds since the Unix epoch, used for timestamp-based versioning.
	tidbGenerateTimestampVersion = `CAST(UNIX_TIMESTAMP(NOW(6)) * 1000000 AS SIGNED)`

	postgresGenerateTimestampVersion = `(EXTRACT(EPOCH FROM now()) * 1000000)::INT8`
	spannerGenerateTimestampVersion  = `UNIX_MICROS(CURRENT_TIMESTAMP())`

	// tidbMaxSegmentBatch caps the number of segments included in a single
	// multi-row statement (INSERT, DELETE … IN(…), UPDATE … JOIN …). The
	// driver runs with interpolateParams=true, so the binary-protocol uint16
	// placeholder cap doesn't apply; the practical ceilings are TiDB's
	// `max_allowed_packet` (64 MiB) and `txn-entry-size-limit` (6 MiB).
	tidbMaxSegmentBatch = 5000
)

// retryVersionConflict re-runs fn when it fails with a database specific
// version conflict error. Writes that place an object at a freshly computed
// version can race with a concurrent writer at the same location; fn must
// only wrap operations whose primary key collision can solely be caused by
// such a version race, so that re-running fn recomputes the version and
// converges.
func retryVersionConflict(ctx context.Context, fn func(ctx context.Context) error) error {
	return tidbRetryVersionConflict(ctx, fn)
}

// generateVersion generates the SQL snippet to get a new version for an object,
// assuming (project_id, bucket_name, object_key) are provided as the first three parameters.
func (p *PostgresAdapter) generateVersion() string {
	if p.config.TestingTimestampVersioning {
		return postgresGenerateTimestampVersion
	}
	return postgresGenerateNextVersion
}

// generateVersion generates the SQL snippet to get a new version for an object,
// assuming (project_id, bucket_name, object_key) are provided as parameters named @project_id, @bucket_name, @object_key.
func (p *SpannerAdapter) generateVersion() string {
	if p.config.TestingTimestampVersioning {
		return spannerGenerateTimestampVersion
	}
	return spannerGenerateNextVersion
}

func nextVersion(pendingVersion, highestVersion, timestampVersion Version, testingTimestampVersioning bool) (result Version) {
	pendingVersion = max(pendingVersion, 0)
	highestVersion = max(highestVersion, 0)

	if pendingVersion == highestVersion {
		if pendingVersion == 0 {
			return 1
		}
		return pendingVersion
	}

	if testingTimestampVersioning {
		if highestVersion > timestampVersion {
			// this should never happen, but just in case
			return highestVersion + 1
		}
		if timestampVersion == 0 {
			return 1
		}
		return timestampVersion
	}

	return highestVersion + 1
}
