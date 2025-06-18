// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"

	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil/pgutil"
)

// TestMigrateToLatest creates a database and applies all the migration for test purposes.
func (p *PostgresAdapter) TestMigrateToLatest(ctx context.Context) error {
	schema, err := pgutil.ParseSchemaFromConnstr(p.connstr)
	if err != nil {
		return errs.New("error parsing schema: %+v", err)
	}

	if schema != "" {
		err = pgutil.CreateSchema(ctx, p.db, schema)
		if err != nil {
			return errs.New("error creating schema: %+v", err)
		}
	}
	return p.testMigrateToLatest(ctx)
}

func (p *PostgresAdapter) testMigrateToLatest(ctx context.Context) error {
	migration := &migrate.Migration{
		Table: "metabase_versions",
		Steps: []*migrate.Step{
			{
				DB:          &p.db,
				Description: "Test snapshot",
				Version:     21,
				Action: migrate.SQL{
					`CREATE TABLE objects (
						project_id   BYTEA NOT NULL,
						bucket_name  BYTEA NOT NULL, -- we're using bucket_name here to avoid a lookup into buckets table
						object_key   BYTEA NOT NULL, -- using 'object_key' instead of 'key' to avoid reserved word
						version      INT8  NOT NULL,
						stream_id    BYTEA NOT NULL,

						product_id INTEGER,

						created_at TIMESTAMPTZ NOT NULL default now(),
						expires_at TIMESTAMPTZ,

						status         INT2 NOT NULL default ` + statusPending + `,
						segment_count  INT4 NOT NULL default 0,

						encrypted_metadata_nonce         BYTEA default NULL,
						encrypted_metadata               BYTEA default NULL,
						encrypted_metadata_encrypted_key BYTEA default NULL,
						encrypted_etag                   BYTEA default NULL,

						total_plain_size     INT8 NOT NULL default 0, -- migrated objects have this = 0
						total_encrypted_size INT8 NOT NULL default 0,
						fixed_segment_size   INT4 NOT NULL default 0, -- migrated objects have this = 0

						encryption INT8 NOT NULL default 0,

						zombie_deletion_deadline TIMESTAMPTZ default now() + '1 day',

						retention_mode INT2,
						retain_until   TIMESTAMPTZ,

						PRIMARY KEY (project_id, bucket_name, object_key, version)
					);

					COMMENT ON TABLE  objects             is 'Objects table contains information about path and streams.';
					COMMENT ON COLUMN objects.project_id  is 'project_id is a uuid referring to project.id.';
					COMMENT ON COLUMN objects.bucket_name is 'bucket_name is a alpha-numeric string referring to bucket_metainfo.name.';
					COMMENT ON COLUMN objects.object_key  is 'object_key is an encrypted path of the object.';
					COMMENT ON COLUMN objects.version     is 'version is a monotonically increasing number per object. currently unused.';
					COMMENT ON COLUMN objects.stream_id   is 'stream_id is a random identifier for the content uploaded to the object.';

					COMMENT ON COLUMN objects.product_id is 'product_id specifies which product the object is.';

					COMMENT ON COLUMN objects.created_at  is 'created_at is the creation date of this object.';
					COMMENT ON COLUMN objects.expires_at  is 'expires_at is the date when this object will be marked for deletion.';

					COMMENT ON COLUMN objects.status        is 'status refers to metabase.ObjectStatus, where pending=1 and committed=3.';
					COMMENT ON COLUMN objects.segment_count is 'segment_count indicates, how many segments are in the segments table for this object. This is zero until the object is committed.';

					COMMENT ON COLUMN objects.encrypted_metadata_nonce is 'encrypted_metadata_nonce is random identifier used as part of encryption for encrypted_metadata.';
					COMMENT ON COLUMN objects.encrypted_metadata       is 'encrypted_metadata is encrypted key-value pairs of user-specified data.';
					COMMENT ON COLUMN objects.encrypted_metadata_encrypted_key is 'encrypted_metadata_encrypted_key is the encrypted key for encrypted_metadata.';
					COMMENT ON COLUMN objects.encrypted_etag           is 'encrypted_etag is the etag, which has been encrypted.';

					COMMENT ON COLUMN objects.total_plain_size     is 'total_plain_size is the user-specified total size of the object. This can be zero for old migrated objects.';
					COMMENT ON COLUMN objects.total_encrypted_size is 'total_encrypted_size is the sum of the encrypted data sizes of segments.';
					COMMENT ON COLUMN objects.fixed_segment_size   is 'fixed_segment_size is specified for objects that have a uniform segment sizes (except the last segment). This can be zero for old migrated objects.';

					COMMENT ON COLUMN objects.encryption is 'encryption contains object encryption parameters encoded into a uint32. See metabase.encryptionParameters type for the implementation.';

					COMMENT ON COLUMN objects.zombie_deletion_deadline is 'zombie_deletion_deadline defines when a pending object can be deleted due to a failed upload.';

					COMMENT ON COLUMN objects.retention_mode is 'retention_mode specifies an object version''s retention mode: NULL/0=none, and 1=compliance.';
					COMMENT ON COLUMN objects.retain_until   is 'retain_until specifies when an object version''s retention period ends.';

					CREATE TABLE segments (
						stream_id  BYTEA NOT NULL,
						position   INT8  NOT NULL,

						root_piece_id       BYTEA NOT NULL,
						encrypted_key_nonce BYTEA NOT NULL,
						encrypted_key       BYTEA NOT NULL,
						remote_alias_pieces BYTEA,

						encrypted_size INT4 NOT NULL,
						plain_offset   INT8 NOT NULL, -- migrated objects have this = 0
						plain_size     INT4 NOT NULL, -- migrated objects have this = 0

						redundancy INT8 NOT NULL default 0,

						inline_data  BYTEA DEFAULT NULL,

						created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
						repaired_at TIMESTAMPTZ,
						expires_at TIMESTAMPTZ,

						placement integer,
						encrypted_etag BYTEA default NULL,

						PRIMARY KEY (stream_id, position)
					);

					COMMENT ON TABLE  segments            is 'segments table contains where segment data is located and other metadata about them.';
					COMMENT ON COLUMN segments.stream_id  is 'stream_id is a uuid referring to segments that belong to the same object.';
					COMMENT ON COLUMN segments.position   is 'position is a segment sequence number, determining the order they should be read in. It is represented as uint64, where the upper 32bits indicate the part-number and the lower 32bits indicate the index inside the part.';

					COMMENT ON COLUMN segments.root_piece_id       is 'root_piece_id is used for deriving per storagenode piece numbers.';
					COMMENT ON COLUMN segments.encrypted_key_nonce is 'encrypted_key_nonce is random data used for encrypting the encrypted_key.';
					COMMENT ON COLUMN segments.encrypted_key       is 'encrypted_key is the encrypted key that was used for encrypting the data in this segment.';
					COMMENT ON COLUMN segments.remote_alias_pieces is 'remote_alias_pieces is a compressed list of storagenodes that contain the pieces. See metabase.AliasPieces to see how they are compressed.';

					COMMENT ON COLUMN segments.encrypted_size is 'encrypted_size is the data size after compression, but before Reed-Solomon encoding.';
					COMMENT ON COLUMN segments.plain_offset   is 'plain_offset is the offset of this segment in the unencrypted data stream. Old migrated objects do not have this information, and is zero.';
					COMMENT ON COLUMN segments.plain_size     is 'plain_size is the user-specified unencrypted size of this segment. Old migrated objects do not have this information, and it is zero.';

					COMMENT ON COLUMN segments.redundancy  is 'redundancy is the compressed Reed-Solomon redundancy parameters for this segment. See metabase.redundancyScheme for the compression.';

					COMMENT ON COLUMN segments.inline_data is 'inline_data contains encrypted data for small objects.';

					COMMENT ON COLUMN segments.created_at  is 'created_at is the date when the segment was committed to the table.';
					COMMENT ON COLUMN segments.repaired_at is 'repaired_at is the last date when the segment was repaired.';
					COMMENT ON COLUMN segments.expires_at  is 'expires_at is the date when the segment is marked for deletion.';

					COMMENT ON COLUMN segments.placement is 'placement is the country or region restriction for the segment data. See storj.PlacementConstraint for the values.';
					COMMENT ON COLUMN segments.encrypted_etag is 'encrypted_etag is etag that has been encrypted.';

					CREATE SEQUENCE node_alias_seq
						INCREMENT BY 1
						MINVALUE 1 MAXVALUE 2147483647 -- MaxInt32
						START WITH 1;
					CREATE TABLE node_aliases (
						node_id    BYTEA  NOT NULL UNIQUE,
						node_alias INT4   NOT NULL UNIQUE default nextval('node_alias_seq')
					);

					COMMENT ON TABLE  node_aliases            is 'node_aliases table contains unique identifiers (aliases) for storagenodes that take less space than a NodeID.';
					COMMENT ON COLUMN node_aliases.node_id    is 'node_id refers to the storj.NodeID';
					COMMENT ON COLUMN node_aliases.node_alias is 'node_alias is a unique integer value assigned for the node_id. It is used for compressing segments.remote_alias_pieces.';`,
				},
			},
		},
	}

	if p.testingUniqueUnversioned {
		// This is only part of testing, because we do not want to affect the production performance.
		migration.Steps = append(migration.Steps, &migrate.Step{
			DB:          &p.db,
			Description: "Constraint for ensuring our metabase correctness.",
			Version:     22,
			Action: migrate.SQL{
				`CREATE UNIQUE INDEX objects_one_unversioned_per_location ON objects (project_id, bucket_name, object_key) WHERE status IN ` + statusesUnversioned + `;`,
			},
		})
	}

	return migration.Run(ctx, p.log.Named("migrate"))
}

// TestMigrateToLatest creates a database and applies all the migration for test purposes.
func (c *CockroachAdapter) TestMigrateToLatest(ctx context.Context) error {
	var dbName string
	if err := c.db.QueryRowContext(ctx, `SELECT current_database();`).Scan(&dbName); err != nil {
		return errs.New("error querying current database: %+v", err)
	}

	_, err := c.db.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s;`,
		pgutil.QuoteIdentifier(dbName)))
	if err != nil {
		return errs.Wrap(err)
	}

	return c.PostgresAdapter.testMigrateToLatest(ctx)
}

// TestMigrateToLatest creates a database and applies all the migration for test purposes.
func (s *SpannerAdapter) TestMigrateToLatest(ctx context.Context) error {
	var statements []string
	for _, ddl := range strings.Split(spannerDDL, ";") {
		if strings.TrimSpace(ddl) != "" {
			statements = append(statements, ddl)
		}
	}

	operation, err := s.adminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   s.connParams.DatabasePath(),
		Statements: statements,
	})
	if err != nil {
		return errs.Wrap(err)
	}

	return operation.Wait(ctx)
}
