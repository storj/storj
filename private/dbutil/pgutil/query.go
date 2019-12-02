// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"fmt"
	"regexp"

	"github.com/lib/pq"
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil/dbschema"
)

// QuerySchema loads the schema from postgres database.
func QuerySchema(db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	// find tables
	err := func() error {
		rows, err := db.Query(`
			SELECT table_name, column_name, is_nullable, data_type
			FROM  information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var tableName, columnName, isNullable, dataType string
			err := rows.Scan(&tableName, &columnName, &isNullable, &dataType)
			if err != nil {
				return err
			}

			table := schema.EnsureTable(tableName)
			table.AddColumn(&dbschema.Column{
				Name:       columnName,
				Type:       dataType,
				IsNullable: isNullable == "YES",
			})
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	// find constraints
	err = func() error {
		// TODO: its typically better to use explicit JOIN, however until cockroachdb releases
		// support for `JOIN LATERAL` we need to use implicit JOIN syntax.
		// ref: https://github.com/cockroachdb/cockroach/pull/40945
		// This query is being left here commented out so that we can easily add it back in.

		// rows, err := db.Query(`
		// 	SELECT pg_class.relname,
		// 		pg_constraint.conname,
		// 		pg_constraint.contype,
		// 		ARRAY_AGG(pg_attribute.attname ORDER BY u.attposition),
		// 		pg_get_constraintdef(pg_constraint.oid)
		// 	FROM pg_constraint
		// 		JOIN LATERAL UNNEST(pg_constraint.conkey) WITH ORDINALITY AS u(attnum, attposition) ON TRUE
		// 		JOIN pg_class ON pg_class.oid = pg_constraint.conrelid
		// 		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		// 		JOIN pg_attribute ON (pg_attribute.attrelid = pg_class.oid AND pg_attribute.attnum = u.attnum)
		// 	WHERE pg_namespace.nspname = CURRENT_SCHEMA
		// 	GROUP BY pg_constraint.conname, pg_constraint.contype, pg_class.relname, pg_get_constraintdef(pg_constraint.oid);
		// `)
		rows, err := db.Query(`
			SELECT pg_class.relname,
				pg_constraint.conname,
				pg_constraint.contype,
				ARRAY_AGG(pg_attribute.attname ORDER BY u.attposition),
				pg_get_constraintdef(pg_constraint.oid)
			FROM pg_constraint, UNNEST(pg_constraint.conkey) WITH ORDINALITY AS u(attnum, attposition),
				pg_class,
				pg_namespace,
				pg_attribute
			WHERE pg_namespace.nspname = CURRENT_SCHEMA
				AND pg_class.oid = pg_constraint.conrelid
				AND pg_namespace.oid = pg_class.relnamespace
				AND pg_attribute.attrelid = pg_class.oid
				AND pg_attribute.attnum = u.attnum
			GROUP BY pg_constraint.conname, pg_constraint.contype, pg_class.relname, pg_get_constraintdef(pg_constraint.oid);
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var tableName, constraintName, constraintType string
			var columns pq.StringArray
			var definition string

			err := rows.Scan(&tableName, &constraintName, &constraintType, &columns, &definition)
			if err != nil {
				return err
			}

			switch constraintType {
			case "p": // primary key
				table := schema.EnsureTable(tableName)
				table.PrimaryKey = ([]string)(columns)
			case "f": // foreign key
				if len(columns) != 1 {
					return fmt.Errorf("expected one column, got: %q", columns)
				}

				table := schema.EnsureTable(tableName)
				column, ok := table.FindColumn(columns[0])
				if !ok {
					return fmt.Errorf("did not find column %q", columns[0])
				}

				matches := rxPostgresForeignKey.FindStringSubmatch(definition)
				if len(matches) == 0 {
					return fmt.Errorf("unable to parse constraint %q", definition)
				}

				column.Reference = &dbschema.Reference{
					Table:    matches[1],
					Column:   matches[2],
					OnUpdate: matches[3],
					OnDelete: matches[4],
				}
			case "u": // unique
				table := schema.EnsureTable(tableName)
				table.Unique = append(table.Unique, columns)
			default:
				return fmt.Errorf("unhandled constraint type %q", constraintType)
			}
		}
		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	// TODO: find indexes
	schema.Sort()
	return schema, nil
}

// matches FOREIGN KEY (project_id) REFERENCES projects(id) ON UPDATE CASCADE ON DELETE CASCADE
var rxPostgresForeignKey = regexp.MustCompile(
	`^FOREIGN KEY \([[:word:]]+\) ` +
		`REFERENCES ([[:word:]]+)\(([[:word:]]+)\)` +
		`(?:\s*ON UPDATE (CASCADE|RESTRICT|SET NULL|SET DEFAULT|NO ACTION))?` +
		`(?:\s*ON DELETE (CASCADE|RESTRICT|SET NULL|SET DEFAULT|NO ACTION))?$`,
)
