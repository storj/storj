// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgtype"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbschema"
)

// QuerySchema loads the schema from postgres database.
func QuerySchema(ctx context.Context, db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	// get version string to do efficient queries
	var version string
	row := db.QueryRowContext(ctx, `SELECT version()`)
	if err := row.Scan(&version); err != nil {
		return nil, errs.Wrap(err)
	}

	// find sequences
	err := func() (err error) {
		rows, err := db.QueryContext(ctx, "SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = CURRENT_SCHEMA")
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			if err != nil {
				return err
			}

			schema.Sequences = append(schema.Sequences, name)
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}
	sort.Strings(schema.Sequences)

	// find tables
	err = func() (err error) {
		rows, err := db.QueryContext(ctx, `
			SELECT table_name, column_name, is_nullable, coalesce(column_default, ''), data_type
			FROM  information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var tableName, columnName, isNullable, columnDefault, dataType string
			err := rows.Scan(&tableName, &columnName, &isNullable, &columnDefault, &dataType)
			if err != nil {
				return err
			}
			// cockroach may have a (not so) hidden table for the sequence which should be ignored
			if schema.HasSequence(tableName) {
				continue
			}

			table := schema.EnsureTable(tableName)
			table.AddColumn(&dbschema.Column{
				Name:       columnName,
				Type:       dataType,
				IsNullable: isNullable == "YES",
				Default:    parseColumnDefault(columnDefault),
			})
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	// find constraints
	err = func() (err error) {
		// cockroach has a .condef field and it's way faster than the function call
		var definitionClause string
		if strings.Contains(version, "CockroachDB") {
			definitionClause = `pg_constraint.condef AS definition`
		} else {
			definitionClause = `pg_get_constraintdef(pg_constraint.oid) AS definition`
		}

		rows, err := db.QueryContext(ctx, `
			SELECT
				pg_class.relname      AS table_name,
				pg_constraint.conname AS constraint_name,
				pg_constraint.contype AS constraint_type,
				(
					SELECT
						ARRAY_AGG(pg_attribute.attname ORDER BY u.pos)
					FROM
						pg_attribute
						JOIN UNNEST(pg_constraint.conkey) WITH ORDINALITY AS u(attnum, pos) ON u.attnum = pg_attribute.attnum
					WHERE
						pg_attribute.attrelid = pg_class.oid
				) AS columns, `+definitionClause+`
			FROM
				pg_constraint
				JOIN pg_class ON pg_class.oid = pg_constraint.conrelid
				JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
			WHERE pg_namespace.nspname = CURRENT_SCHEMA
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var tableName, constraintName, constraintType string
			var columnsArray pgtype.VarcharArray
			var columns []string
			var definition string

			err := rows.Scan(&tableName, &constraintName, &constraintType, &columnsArray, &definition)
			if err != nil {
				return err
			}

			if schema.HasSequence(tableName) {
				continue
			}

			err = columnsArray.AssignTo(&columns)
			if err != nil {
				return err
			}

			switch constraintType {
			case "p": // primary key
				table := schema.EnsureTable(tableName)
				table.PrimaryKey = columns
			case "f": // foreign key
				table := schema.EnsureTable(tableName)

				// All foreign keys (single and composite) are now stored in Table.ForeignKeys
				matches := rxPostgresCompositeForeignKey.FindStringSubmatch(definition)
				if len(matches) == 0 {
					return fmt.Errorf("unable to parse foreign key constraint %q", definition)
				}

				// Parse foreign columns from matches[3], splitting by comma and trimming spaces
				foreignColumnsRaw := strings.Split(matches[3], ",")
				foreignColumns := make([]string, len(foreignColumnsRaw))
				for i, col := range foreignColumnsRaw {
					foreignColumns[i] = strings.TrimSpace(col)
				}

				table.ForeignKeys = append(table.ForeignKeys, &dbschema.ForeignKey{
					Name:           constraintName,
					LocalColumns:   columns,
					ForeignTable:   matches[2],
					ForeignColumns: foreignColumns,
					OnUpdate:       matches[4],
					OnDelete:       matches[5],
				})
			case "u": // unique
				table := schema.EnsureTable(tableName)
				table.Unique = append(table.Unique, columns)
			case "c": // unique
				table := schema.EnsureTable(tableName)

				// workaround for psql vs crdb differences
				definition = strings.ReplaceAll(definition, "!=", "<>")
				table.Checks = append(table.Checks, definition)
			default:
				return errs.New("unhandled constraint type %q", constraintType)
			}
		}
		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	// find indexes
	err = func() (err error) {
		rows, err := db.QueryContext(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname = CURRENT_SCHEMA`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var indexdef string
			err := rows.Scan(&indexdef)
			if err != nil {
				return errs.Wrap(err)
			}

			index, err := parseIndexDefinition(indexdef)
			if err != nil {
				return errs.Wrap(err)
			}
			if isAutogeneratedCockroachIndex(index) {
				continue
			}

			if schema.HasSequence(index.Table) {
				continue
			}

			schema.Indexes = append(schema.Indexes, index)
		}

		return errs.Wrap(rows.Err())
	}()
	if err != nil {
		return nil, err
	}

	schema.Sort()
	return schema, nil
}

// rxPostgresCompositeForeignKey matches composite (multi-column) foreign key constraints
var rxPostgresCompositeForeignKey = regexp.MustCompile(
	`^FOREIGN KEY \(([^)]+)\) ` +
		`REFERENCES ([[:word:]]+)\(([^)]+)\)` +
		`(?:\s*ON UPDATE (CASCADE|RESTRICT|SET NULL|SET DEFAULT|NO ACTION))?` +
		`(?:\s*ON DELETE (CASCADE|RESTRICT|SET NULL|SET DEFAULT|NO ACTION))?$`,
)

var (
	rxIndex                  = regexp.MustCompile(`^CREATE( UNIQUE)? INDEX (.*) ON .*\.(.*) USING btree \(([^)]+)\)(?: STORING \([^)]+\))?(?: WHERE (.+))?`)
	indexDirNullsOrderRemove = strings.NewReplacer(" ASC", "", " DESC", "", " NULLS", "", " FIRST", "", " LAST", "")
	typeDescriptorRx         = regexp.MustCompile(`::(:)?[a-zA-Z0-9_ ]+`)
)

func parseColumnDefault(columnDefault string) string {
	// hackity hack: See the comments in parseIndexDefinition for why we do this.
	if columnDefault == "nextval('storagenode_storage_tallies_id_seq'::regclass)" {
		return "nextval('accounting_raws_id_seq'::regclass)"
	}

	// hackity hack: cockroach sometimes adds type descriptors to the default. ignore em!
	columnDefault = typeDescriptorRx.ReplaceAllString(columnDefault, "")

	return columnDefault
}

func parseIndexDefinition(indexdef string) (*dbschema.Index, error) {
	matches := rxIndex.FindStringSubmatch(indexdef)
	if matches == nil {
		return nil, errs.New("unable to parse index (you should go make the parser better): %q", indexdef)
	}

	// hackity hack: cockroach returns all primary key index names as `"primary"`, but sometimes
	// our migrations create them with explicit names. so let's change all of them.
	name := matches[2]
	if name == `"primary"` {
		name = matches[3] + "_pkey"
	}

	// hackity hack: sometimes they end with _pk, sometimes they end with _pkey. let's make them
	// all end with _pkey.
	if strings.HasSuffix(name, "_pk") {
		name = name[:len(name)-3] + "_pkey"
	}

	// biggest hackity hack of all: we apparently did
	//
	//     CREATE TABLE accounting_raws ( ... )
	//     ALTER TABLE accounting_raws RENAME TO storagenode_storage_tallies
	//
	// but that means the primary key index is still named accounting_raws_pkey and not
	// the expected storagenode_storage_tallies_pkey.
	//
	// "No big deal", you might say, "just add an ALTER INDEX". Ah, but recall: cockroach
	// does not name their primary key indexes. They are instead all named `"primary"`.
	// Now, at this point, a clever person might suggest ALTER INDEX IF EXISTS so that
	// it renames it on postgres but not cockroach. Surely if the index does not exist
	// it will happily succeed. You'd like to think that, wouldn't you! Alas, cockroach
	// will error on ALTER INDEX IF EXISTS even if the index does not exist. Basic
	// conditionals are apparently too hard for it.
	//
	// Undaunted, I searched their bug tracker and found this comment within this issue:
	//     https://github.com/cockroachdb/cockroach/issues/42399#issuecomment-558377915
	// It turns out, you apparently need to specify the index with some sort of `@` sigil
	// or it just errors with an unhelpful message. But only a great fool would think that
	// the query would remain valid for postgres!
	//
	// In summary, because cockroach errors even if the index does not exist, I can clearly
	// not use cockroach. But because postgres will error if the sigil is included, I can
	// clearly not use postgres.
	//
	// As a last resort, one may suggest changing the postgres.N.sql file to ALSO create
	// the wrong table name and rename it. Truly, they have a dizzying intellect. But prepare
	// yourself for the final killing blow: if we do that, then the final output does not match
	// the dbx schema that is autogenerated, and the test still fails.
	//
	// The lesson? Never go in against a database when death is on the line. HA HA HA HA...
	//
	// Bleh.
	if name == "accounting_raws_pkey" {
		name = "storagenode_storage_tallies_pkey"
	}

	columns := strings.Split(indexDirNullsOrderRemove.Replace(matches[4]), ", ")
	for i, column := range columns {
		columns[i] = UnquoteIdentifier(column)
	}

	return &dbschema.Index{
		Name:    name,
		Table:   matches[3],
		Unique:  matches[1] != "",
		Columns: columns,
		Partial: matches[5],
	}, nil
}

// hackity hack:
//
// Cockroach sometimes creates automatic indexes to enforce foreign key
// relationships, if it doesn't think the need is already met by other
// indexes. If you then add the other indexes after creating the table,
// the auto-generated index does not go away. So you get different results
// when establishing one table with a set of constraints over multiple
// steps, versus creating that same table with the same set of constraints
// all at once. Unfortunately, our system wants very much for those two
// paths to produce exactly the same result.
//
// This should make it so that we disregard the difference in the cases
// that it arises.
//
// See above for an important lesson about going in against a database when
// death is on the line.
func isAutogeneratedCockroachIndex(index *dbschema.Index) bool {
	return strings.Contains(index.Name, "_auto_index_fk_")
}
