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
		rows, err := db.Query(`
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
				) AS columns,
				pg_get_constraintdef(pg_constraint.oid) AS definition
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
