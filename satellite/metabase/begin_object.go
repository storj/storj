// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	pgxerrcode "github.com/jackc/pgerrcode"
	"google.golang.org/grpc/codes"

	"storj.io/common/storj"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
)

// BeginObjectNextVersion contains arguments necessary for starting an object upload.
type BeginObjectNextVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedUserData
	Encryption storj.EncryptionParameters

	Retention Retention // optional
	LegalHold bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify verifies get object request fields.
func (opts *BeginObjectNextVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version != NextVersion {
		return ErrInvalidRequest.New("Version should be metabase.NextVersion")
	}

	err := opts.EncryptedUserData.VerifyForBegin()
	if err != nil {
		return err
	}

	if err := opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if opts.ExpiresAt != nil {
		switch {
		case opts.Retention.Enabled():
			return ErrInvalidRequest.New("ExpiresAt must not be set if Retention is set")
		case opts.LegalHold:
			return ErrInvalidRequest.New("ExpiresAt must not be set if LegalHold is set")
		}
	}

	return nil
}

// BeginObjectNextVersion adds a pending object to the database, with automatically assigned version.
func (db *DB) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (object Object, err error) {
	return db.ChooseAdapter(opts.ProjectID).BeginObjectNextVersion(ctx, opts)
}

func beginObjectNextVersion(ctx context.Context, adapterFunc func(context.Context, BeginObjectNextVersion, *Object) error, opts BeginObjectNextVersion) (object Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if err := opts.Verify(); err != nil {
		return Object{}, err
	}

	if opts.ZombieDeletionDeadline == nil {
		deadline := time.Now().Add(defaultZombieDeletionPeriod)
		opts.ZombieDeletionDeadline = &deadline
	}

	object = Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			StreamID:   opts.StreamID,
		},
		Status:                 DefaultStatus,
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		EncryptedUserData:      opts.EncryptedUserData,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
		Retention:              opts.Retention,
		LegalHold:              opts.LegalHold,
	}

	if err := adapterFunc(ctx, opts, &object); err != nil {
		return Object{}, Error.New("unable to insert object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginObjectNextVersion adds a pending object to the database, with automatically assigned version.
func (p *PostgresAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (object Object, err error) {
	return beginObjectNextVersion(ctx, p.beginObjectNextVersion, opts)
}

// BeginObjectNextVersion adds a pending object to the database, with automatically assigned version.
func (s *SpannerAdapter) BeginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion) (object Object, err error) {
	return beginObjectNextVersion(ctx, s.beginObjectNextVersion, opts)
}

// BeginObjectNextVersion implements Adapter.
func (p *PostgresAdapter) beginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	return p.db.QueryRowContext(ctx, `
			INSERT INTO objects (
				project_id, bucket_name, object_key, version, stream_id,
				expires_at, encryption,
				zombie_deletion_deadline,
				encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
				checksum,
				retention_mode, retain_until
			) VALUES (
				$1, $2, $3, `+p.generateVersion()+`,
				$4, $5, $6,
				$7,
				$8, $9, $10, $11,
				$12,
				$13, $14
			)
			RETURNING version, created_at
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.StreamID,
		opts.ExpiresAt, opts.Encryption,
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey, opts.EncryptedETag,
		opts.Checksum,
		lockModeWrapper{
			retentionMode: &opts.Retention.Mode,
			legalHold:     &opts.LegalHold,
		}, timeWrapper{&opts.Retention.RetainUntil},
	).Scan(&object.Version, &object.CreatedAt)
}

// BeginObjectNextVersion implements Adapter.
func (s *SpannerAdapter) beginObjectNextVersion(ctx context.Context, opts BeginObjectNextVersion, object *Object) error {
	object.CreatedAt = time.Now()
	_, err := s.client.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		return Error.Wrap(txn.Query(ctx, spanner.Statement{
			SQL: `INSERT objects (
					project_id, bucket_name, object_key, version, stream_id,
					created_at,expires_at, encryption,
					zombie_deletion_deadline,
					encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
					checksum,
					retention_mode, retain_until
				) VALUES (
					@project_id, @bucket_name, @object_key,
					` + s.generateVersion() + `,
					@stream_id, @created_at, @expires_at,
					@encryption, @zombie_deletion_deadline,
					@encrypted_metadata, @encrypted_metadata_nonce, @encrypted_metadata_encrypted_key, @encrypted_etag,
					@checksum,
					@retention_mode, @retain_until
				)
				THEN RETURN version`,
			Params: map[string]any{
				"project_id":                       opts.ProjectID,
				"bucket_name":                      opts.BucketName,
				"object_key":                       opts.ObjectKey,
				"stream_id":                        opts.StreamID,
				"created_at":                       object.CreatedAt,
				"expires_at":                       opts.ExpiresAt,
				"encryption":                       opts.Encryption,
				"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
				"encrypted_metadata":               opts.EncryptedMetadata,
				"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
				"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
				"encrypted_etag":                   opts.EncryptedETag,
				"checksum":                         opts.Checksum,
				"retention_mode": lockModeWrapper{
					retentionMode: &opts.Retention.Mode,
					legalHold:     &opts.LegalHold,
				},
				"retain_until": timeWrapper{&opts.Retention.RetainUntil},
			},
		}).Do(func(row *spanner.Row) error {
			return Error.Wrap(row.Columns(&object.Version))
		}))
	}, spanner.TransactionOptions{
		CommitOptions: spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		},
		TransactionTag:              "begin-object-next-version",
		ExcludeTxnFromChangeStreams: true,
	})
	return err
}

// BeginObjectExactVersion contains arguments necessary for starting an object upload.
type BeginObjectExactVersion struct {
	ObjectStream

	ExpiresAt              *time.Time
	ZombieDeletionDeadline *time.Time

	EncryptedUserData
	Encryption storj.EncryptionParameters

	Retention Retention // optional
	LegalHold bool

	// TestingBypassVerify makes the (*DB).TestingBeginObjectExactVersion method skip
	// validation of this struct's fields. This is useful for inserting intentionally
	// malformed or unexpected data into the database and testing that we handle it properly.
	TestingBypassVerify bool

	// supported only by Spanner.
	MaxCommitDelay *time.Duration
}

// Verify verifies get object reqest fields.
func (opts *BeginObjectExactVersion) Verify() error {
	if err := opts.ObjectStream.Verify(); err != nil {
		return err
	}

	if opts.Version == NextVersion {
		return ErrInvalidRequest.New("Version should not be metabase.NextVersion")
	}

	err := opts.EncryptedUserData.VerifyForBegin()
	if err != nil {
		return err
	}

	if err := opts.Retention.Verify(); err != nil {
		return ErrInvalidRequest.Wrap(err)
	}

	if opts.ExpiresAt != nil {
		switch {
		case opts.Retention.Enabled():
			return ErrInvalidRequest.New("ExpiresAt must not be set if Retention is set")
		case opts.LegalHold:
			return ErrInvalidRequest.New("ExpiresAt must not be set if LegalHold is set")
		}
	}

	return nil
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (db *DB) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (committed Object, err error) {
	return db.ChooseAdapter(opts.ProjectID).BeginObjectExactVersion(ctx, opts)
}

func beginObjectExactVersion(ctx context.Context, adapterFunc func(ctx context.Context, opts BeginObjectExactVersion, object *Object) error, opts BeginObjectExactVersion) (committed Object, err error) {
	defer mon.Task()(&ctx)(&err)

	if !opts.TestingBypassVerify {
		if err := opts.Verify(); err != nil {
			return Object{}, err
		}
	}

	if opts.ZombieDeletionDeadline == nil {
		deadline := time.Now().Add(defaultZombieDeletionPeriod)
		opts.ZombieDeletionDeadline = &deadline
	}

	object := Object{
		ObjectStream: ObjectStream{
			ProjectID:  opts.ProjectID,
			BucketName: opts.BucketName,
			ObjectKey:  opts.ObjectKey,
			Version:    opts.Version,
			StreamID:   opts.StreamID,
		},
		Status:                 DefaultStatus,
		ExpiresAt:              opts.ExpiresAt,
		Encryption:             opts.Encryption,
		EncryptedUserData:      opts.EncryptedUserData,
		ZombieDeletionDeadline: opts.ZombieDeletionDeadline,
		Retention:              opts.Retention,
		LegalHold:              opts.LegalHold,
	}

	if err := adapterFunc(ctx, opts, &object); err != nil {
		if ErrObjectAlreadyExists.Has(err) {
			return Object{}, err
		}
		return Object{}, Error.New("unable to commit object: %w", err)
	}

	mon.Meter("object_begin").Mark(1)

	return object, nil
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (p *PostgresAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (_ Object, err error) {
	return beginObjectExactVersion(ctx, p.beginObjectExactVersion, opts)
}

// BeginObjectExactVersion adds a pending object to the database, with specific version.
func (s *SpannerAdapter) BeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion) (_ Object, err error) {
	return beginObjectExactVersion(ctx, s.beginObjectExactVersion, opts)
}

func (p *PostgresAdapter) beginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO objects (
			project_id, bucket_name, object_key, version, stream_id,
			expires_at, encryption,
			zombie_deletion_deadline,
			encrypted_metadata, encrypted_metadata_nonce, encrypted_metadata_encrypted_key, encrypted_etag,
			checksum,
			retention_mode, retain_until
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8,
			$9, $10, $11, $12,
			$13,
			$14, $15
		)
		RETURNING created_at
		`, opts.ProjectID, opts.BucketName, opts.ObjectKey, opts.Version, opts.StreamID,
		opts.ExpiresAt, opts.Encryption,
		opts.ZombieDeletionDeadline,
		opts.EncryptedMetadata, opts.EncryptedMetadataNonce, opts.EncryptedMetadataEncryptedKey, opts.EncryptedETag,
		opts.Checksum,
		lockModeWrapper{
			retentionMode: &opts.Retention.Mode,
			legalHold:     &opts.LegalHold,
		}, timeWrapper{&opts.Retention.RetainUntil},
	).Scan(
		&object.CreatedAt,
	)
	if err != nil {
		if code := pgerrcode.FromError(err); code == pgxerrcode.UniqueViolation {
			return Error.Wrap(ErrObjectAlreadyExists.New(""))
		}
	}
	return err
}

func (s *SpannerAdapter) beginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	object.CreatedAt = time.Now()
	_, err := s.client.Apply(ctx, []*spanner.Mutation{
		spanner.InsertMap("objects", map[string]any{
			"project_id":                       opts.ProjectID,
			"bucket_name":                      opts.BucketName,
			"object_key":                       opts.ObjectKey,
			"version":                          opts.Version,
			"stream_id":                        opts.StreamID,
			"created_at":                       object.CreatedAt,
			"expires_at":                       opts.ExpiresAt,
			"encryption":                       opts.Encryption,
			"zombie_deletion_deadline":         opts.ZombieDeletionDeadline,
			"encrypted_metadata":               opts.EncryptedMetadata,
			"encrypted_metadata_nonce":         opts.EncryptedMetadataNonce,
			"encrypted_metadata_encrypted_key": opts.EncryptedMetadataEncryptedKey,
			"encrypted_etag":                   opts.EncryptedETag,
			"checksum":                         opts.Checksum,
			"retention_mode": lockModeWrapper{
				retentionMode: &opts.Retention.Mode,
				legalHold:     &opts.LegalHold,
			},
			"retain_until": timeWrapper{&opts.Retention.RetainUntil},
		}),
	},
		spanner.TransactionTag("begin-object-exact-version"),
		spanner.ExcludeTxnFromChangeStreams(),
		spanner.ApplyCommitOptions(spanner.CommitOptions{
			MaxCommitDelay: opts.MaxCommitDelay,
		}),
	)
	if err != nil {
		if errCode := spanner.ErrCode(err); errCode == codes.AlreadyExists {
			return Error.Wrap(ErrObjectAlreadyExists.New(""))
		}
		return Error.Wrap(err)
	}
	return err
}
