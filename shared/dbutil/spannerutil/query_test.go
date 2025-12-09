// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/dbtest"
)

var dbSetup = `
		CREATE TABLE speakers (
			id INT64 NOT NULL,
			name STRING(MAX) NOT NULL,
			birthdate DATE NOT NULL DEFAULT('1986-05-14'),
			rating FLOAT64,
			best_friend STRING(MAX),
			is_active BOOL NOT NULL,
			login_history ARRAY<BOOL> NOT NULL,
			preferences JSON
		) PRIMARY KEY (id, name);

		CREATE INDEX birthday_idx ON speakers (birthdate);
		CREATE UNIQUE INDEX bff_idx ON speakers (best_friend);

		CREATE SEQUENCE seq1 OPTIONS (sequence_kind='bit_reversed_positive');
		CREATE SEQUENCE seq0 OPTIONS (sequence_kind='bit_reversed_positive');

		CREATE TABLE languages (
			name STRING(MAX) NOT NULL,
			parent STRING(MAX),
			alphabet_type STRING(MAX) NOT NULL,
			added_at TIMESTAMP NOT NULL,
			FOREIGN KEY (parent) REFERENCES languages(name),
		) PRIMARY KEY (name);

		CREATE TABLE letters (
			letter STRING(10) NOT NULL,
			language STRING(MAX) NOT NULL,
			phonetic_sound BYTES(MAX),
			FOREIGN KEY (language) REFERENCES languages(name),
		) PRIMARY KEY (letter, language);

		CREATE TABLE language_events (
			added_at TIMESTAMP NOT NULL,
			admin STRING(MAX) NOT NULL,
			event STRING(MAX) NOT NULL DEFAULT('"foo\""'),
			remote_ip BYTES(16),
			FOREIGN KEY (added_at) REFERENCES languages(added_at)
		) PRIMARY KEY (added_at, event, admin);
	`

func TestQuerySchema(t *testing.T) {
	ctx := testcontext.New(t)

	connstr := dbtest.PickOrStartSpanner(t)
	db, err := OpenUnique(ctx, zaptest.NewLogger(t), connstr, t.Name(), MustSplitSQLStatements(dbSetup))
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	schema, err := QuerySchema(ctx, db)
	require.NoError(t, err)

	require.Equal(t, []string{"seq0", "seq1"}, schema.Sequences)

	require.Equal(t, []*dbschema.Index{
		{
			Name:    "PRIMARY_KEY",
			Table:   "language_events",
			Columns: []string{"added_at", "event", "admin"},
			Unique:  true,
		},
		{
			Name:    "PRIMARY_KEY",
			Table:   "languages",
			Columns: []string{"name"},
			Unique:  true,
		},
		{
			Name:    "PRIMARY_KEY",
			Table:   "letters",
			Columns: []string{"letter", "language"},
			Unique:  true,
		},
		{
			Name:    "PRIMARY_KEY",
			Table:   "speakers",
			Columns: []string{"id", "name"},
			Unique:  true,
		},
		{
			Name:    "bff_idx",
			Table:   "speakers",
			Columns: []string{"best_friend"},
			Unique:  true,
		},
		{
			Name:    "birthday_idx",
			Table:   "speakers",
			Columns: []string{"birthdate"},
			Unique:  false,
		},
	}, schema.Indexes)

	require.Equal(t, []*dbschema.Table{
		{
			Name: "language_events",
			Columns: []*dbschema.Column{
				{
					Name:       "added_at",
					Type:       "timestamp with time zone",
					IsNullable: false,
				},
				{
					Name:       "admin",
					Type:       "text",
					IsNullable: false,
				},
				{
					Name:       "event",
					Type:       "text",
					IsNullable: false,
					Default:    `'"foo\""'`,
				},
				{
					Name:       "remote_ip",
					Type:       "bytes(16)",
					IsNullable: true,
				},
			},
			PrimaryKey: []string{"added_at", "event", "admin"},
			ForeignKeys: []*dbschema.ForeignKey{
				{
					Name:           "FK_language_events_languages_50B8A8E01FA990F9_1",
					LocalColumns:   []string{"added_at"},
					ForeignTable:   "languages",
					ForeignColumns: []string{"added_at"},
					OnDelete:       "",
					OnUpdate:       "",
				},
			},
		},
		{
			Name: "languages",
			Columns: []*dbschema.Column{
				{
					Name:       "added_at",
					Type:       "timestamp with time zone",
					IsNullable: false,
				},
				{
					Name:       "alphabet_type",
					Type:       "text",
					IsNullable: false,
				},
				{
					Name:       "name",
					Type:       "text",
					IsNullable: false,
				},
				{
					Name:       "parent",
					Type:       "text",
					IsNullable: true,
				},
			},
			PrimaryKey: []string{"name"},
			Unique:     [][]string{{"added_at"}},
			ForeignKeys: []*dbschema.ForeignKey{
				{
					Name:           "FK_languages_languages_47C046CFFD9FFD1A_1",
					LocalColumns:   []string{"parent"},
					ForeignTable:   "languages",
					ForeignColumns: []string{"name"},
					OnDelete:       "",
					OnUpdate:       "",
				},
			},
		},
		{
			Name: "letters",
			Columns: []*dbschema.Column{
				{
					Name:       "language",
					Type:       "text",
					IsNullable: false,
				},
				{
					Name:       "letter",
					Type:       "string(10)",
					IsNullable: false,
				},
				{
					Name:       "phonetic_sound",
					Type:       "bytea",
					IsNullable: true,
				},
			},
			PrimaryKey: []string{"letter", "language"},
			ForeignKeys: []*dbschema.ForeignKey{
				{
					Name:           "FK_letters_languages_9130EA5FF4299669_1",
					LocalColumns:   []string{"language"},
					ForeignTable:   "languages",
					ForeignColumns: []string{"name"},
					OnDelete:       "",
					OnUpdate:       "",
				},
			},
		},
		{
			Name: "speakers",
			Columns: []*dbschema.Column{
				{
					Name:       "best_friend",
					Type:       "text",
					IsNullable: true,
				},
				{
					Name:       "birthdate",
					Type:       "date",
					IsNullable: false,
					Default:    "'1986-05-14'",
				},
				{
					Name:       "id",
					Type:       "bigint",
					IsNullable: false,
				},
				{
					Name:       "is_active",
					Type:       "boolean",
					IsNullable: false,
				},
				{
					Name:       "login_history",
					Type:       "boolean[]",
					IsNullable: false,
				},
				{
					Name:       "name",
					Type:       "text",
					IsNullable: false,
				},
				{
					Name:       "preferences",
					Type:       "json",
					IsNullable: true,
				},
				{
					Name:       "rating",
					Type:       "double precision",
					IsNullable: true,
				},
			},
			PrimaryKey: []string{"id", "name"},
			Unique:     [][]string{{"best_friend"}},
		},
	}, schema.Tables)
}
