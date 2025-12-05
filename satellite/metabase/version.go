// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

const (
	postgresGenerateNextVersion = `coalesce((
		SELECT version + 1
		FROM objects
		WHERE (project_id, bucket_name, object_key) = ($1, $2, $3)
		ORDER BY version DESC
		LIMIT 1
	), 1)`
	spannerGenerateNextVersion = `coalesce(
		(SELECT version + 1
		FROM objects
		WHERE (project_id, bucket_name, object_key) = (@project_id, @bucket_name, @object_key)
		ORDER BY version DESC
		LIMIT 1)
	,1)`

	postgresGenerateTimestampVersion = `(EXTRACT(EPOCH FROM now()) * 1000000)::INT8`
	spannerGenerateTimestampVersion  = `UNIX_MICROS(CURRENT_TIMESTAMP())`
)

// generateVersion generates the SQL snippet to get a new version for an object,
// assuming (project_id, bucket_name, object_key) are provided as the first three parameters.
func (p *PostgresAdapter) generateVersion() string {
	if p.testingTimestampVersioning {
		return postgresGenerateTimestampVersion
	}
	return postgresGenerateNextVersion
}

// generateVersion generates the SQL snippet to get a new version for an object,
// assuming (project_id, bucket_name, object_key) are provided as parameters named @project_id, @bucket_name, @object_key.
func (p *SpannerAdapter) generateVersion() string {
	if p.testingTimestampVersioning {
		return spannerGenerateTimestampVersion
	}
	return spannerGenerateNextVersion
}

func (db *DB) nextVersion(pendingVersion, highestVersion, timestampVersion Version) (result Version) {
	pendingVersion = max(pendingVersion, 0)
	highestVersion = max(highestVersion, 0)

	if pendingVersion == highestVersion {
		if pendingVersion == 0 {
			return 1
		}
		return pendingVersion
	}

	if db.config.TestingTimestampVersioning {
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
