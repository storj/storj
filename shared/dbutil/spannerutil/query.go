// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbschema"
)

var (
	autoSpannerIndexRx = regexp.MustCompile(`^IDX_.*_[A-F0-9]{16}$`)
	valueTypePrefixRx  = regexp.MustCompile(`^(ARRAY|BOOL|BYTES|DATE|FLOAT64|INT64|STRING|TIMESTAMP|JSON) (.*)$`)
	sequenceRx         = regexp.MustCompile(`GET_NEXT_SEQUENCE_VALUE\(SEQUENCE (\w+)\)`)
)

// QuerySchema loads the schema from postgres database.
func QuerySchema(ctx context.Context, db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	// find sequences
	err := func() (err error) {
		rows, err := db.QueryContext(ctx, "SELECT name FROM information_schema.sequences WHERE schema = ''")
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

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
			SELECT table_name, column_name, is_nullable, coalesce(column_default, ''), spanner_type
			FROM  information_schema.columns
			WHERE table_schema = ''
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

		for rows.Next() {
			var tableName, columnName, isNullable, columnDefault, dataType string
			err := rows.Scan(&tableName, &columnName, &isNullable, &columnDefault, &dataType)
			if err != nil {
				return err
			}

			columnDefault = parseColumnDefault(columnDefault)

			table := schema.EnsureTable(tableName)
			table.AddColumn(&dbschema.Column{
				Name:       columnName,
				Type:       translateType(dataType),
				IsNullable: isNullable == "YES",
				Default:    columnDefault,
			})
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	// find constraints
	err = func() (err error) {
		rows, err := db.QueryContext(ctx, `
			-- SELECT DISTINCT to handle UNIQUE constraints that appear both in table_constraints and indexes with the same name
			SELECT DISTINCT * FROM (
				SELECT
					c.constraint_name,
					c.table_name,
					c.constraint_type,
					kcu.column_name, -- can't use array_agg because no support for ordering
					kcu.ordinal_position,
					rc.update_rule,
					rc.delete_rule,
					rcu.table_name AS target_table,
					rcu.column_name AS target_column
				FROM
					information_schema.table_constraints c
					LEFT OUTER JOIN information_schema.key_column_usage kcu ON c.constraint_name = kcu.constraint_name AND c.constraint_schema = kcu.constraint_schema
					LEFT OUTER JOIN information_schema.referential_constraints rc ON c.constraint_name = rc.constraint_name AND c.constraint_schema = rc.constraint_schema
					LEFT OUTER JOIN information_schema.key_column_usage rcu ON rc.unique_constraint_name = rcu.constraint_name AND rc.unique_constraint_schema = rcu.constraint_schema AND kcu.ordinal_position = rcu.ordinal_position
				WHERE
					c.table_schema = ''
					AND c.constraint_type != 'CHECK' -- ignore these for now

				-- docs indicate UNIQUE constraints should be represented in table_constraints as well, but they are
				-- present only if created automatically to satisfy a FOREIGN KEY constraint.
				UNION ALL

				SELECT
					i.index_name AS constraint_name,
					i.table_name,
					'UNIQUE' AS constraint_type,
					ic.column_name,
					ic.ordinal_position,
					null AS update_rule,
					null AS delete_rule,
					null AS target_table,
					null AS target_column
				FROM
					information_schema.indexes i
					JOIN information_schema.index_columns ic ON i.index_name = ic.index_name AND i.table_name = ic.table_name AND i.index_type != 'PRIMARY_KEY'
				WHERE
					i.table_schema = ''
					AND i.is_unique
			) ORDER BY table_name, constraint_name, ordinal_position
		`)
		if err != nil {
			return err
		}
		defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

		for rows.Next() {
			var constraintName, tableName, constraintType, columnName string
			var ordinal int64
			var updateRule, deleteRule, targetTable, targetColumn *string

			err := rows.Scan(&constraintName, &tableName, &constraintType, &columnName, &ordinal, &updateRule, &deleteRule, &targetTable, &targetColumn)
			if err != nil {
				return err
			}

			switch constraintType {
			case "PRIMARY KEY":
				table := schema.EnsureTable(tableName)
				if int64(len(table.PrimaryKey)) < ordinal {
					table.PrimaryKey = append(table.PrimaryKey, make([]string, ordinal-int64(len(table.PrimaryKey)))...)
				}
				table.PrimaryKey[ordinal-1] = columnName
			case "FOREIGN KEY":
				table := schema.EnsureTable(tableName)

				if targetTable == nil || targetColumn == nil || updateRule == nil || deleteRule == nil {
					return fmt.Errorf("missing foreign key information for %q", constraintName)
				}

				// Normalize "NO ACTION" to empty string
				onUpdate := *updateRule
				if onUpdate == "NO ACTION" {
					onUpdate = ""
				}
				onDelete := *deleteRule
				if onDelete == "NO ACTION" {
					onDelete = ""
				}

				// Find existing FK or create new one
				var fk *dbschema.ForeignKey
				for _, existing := range table.ForeignKeys {
					if existing.Name == constraintName {
						fk = existing
						break
					}
				}
				if fk == nil {
					fk = &dbschema.ForeignKey{
						Name:           constraintName,
						LocalColumns:   make([]string, 0),
						ForeignTable:   *targetTable,
						ForeignColumns: make([]string, 0),
						OnUpdate:       onUpdate,
						OnDelete:       onDelete,
					}
					table.ForeignKeys = append(table.ForeignKeys, fk)
				}

				// Add this column to the FK (Spanner returns one row per column)
				if int64(len(fk.LocalColumns)) < ordinal {
					fk.LocalColumns = append(fk.LocalColumns, make([]string, ordinal-int64(len(fk.LocalColumns)))...)
					fk.ForeignColumns = append(fk.ForeignColumns, make([]string, ordinal-int64(len(fk.ForeignColumns)))...)
				}
				fk.LocalColumns[ordinal-1] = columnName
				fk.ForeignColumns[ordinal-1] = *targetColumn
			case "UNIQUE":
				table := schema.EnsureTable(tableName)
				if ordinal == 1 {
					table.Unique = append(table.Unique, []string{columnName})
				} else {
					if len(table.Unique) < 1 || int64(len(table.Unique[len(table.Unique)-1])) != (ordinal-1) {
						last := len(table.Unique) - 1
						lastHad := 0
						if last >= 0 {
							lastHad = len(table.Unique[last])
						}
						return fmt.Errorf("expected %d unique columns preceding %q in constraint %q, but found %d", ordinal-1, columnName, constraintName, lastHad)
					}
					table.Unique[len(table.Unique)-1] = append(table.Unique[len(table.Unique)-1], columnName)
				}
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
		rows, err := db.QueryContext(ctx, `
			SELECT
				i.table_name,
				i.index_name,
				i.index_type,
				is_unique,
				is_null_filtered,
				ic.column_name,
				ic.ordinal_position
			FROM
				information_schema.indexes i
				JOIN information_schema.index_columns ic ON i.index_name = ic.index_name AND i.table_name = ic.table_name
			WHERE
				i.table_schema = ''
			ORDER BY i.table_name, i.index_name, ic.ordinal_position
		`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

		for rows.Next() {
			var tableName, indexName, indexType, columnName string
			var isUnique, isNullFiltered bool
			var ordinal int64
			err := rows.Scan(&tableName, &indexName, &indexType, &isUnique, &isNullFiltered, &columnName, &ordinal)
			if err != nil {
				return errs.Wrap(err)
			}

			if autoSpannerIndexRx.MatchString(indexName) {
				// automatically created index
				continue
			}

			if ordinal == 1 {
				schema.Indexes = append(schema.Indexes, &dbschema.Index{
					Name:    indexName,
					Table:   tableName,
					Unique:  isUnique,
					Columns: []string{columnName},
				})
			} else {
				if len(schema.Indexes) == 0 {
					return fmt.Errorf("expected index %q on table %q, got none", indexName, tableName)
				}
				lastIndex := schema.Indexes[len(schema.Indexes)-1]
				if lastIndex.Table != tableName || lastIndex.Name != indexName {
					return fmt.Errorf("expected to be filling in index %q still, but previous index is %q", indexName, lastIndex.Name)
				}
				if int64(len(lastIndex.Columns)) != (ordinal - 1) {
					return fmt.Errorf("expected %d columns preceding %q on %q, got %d", ordinal-1, columnName, indexName, len(lastIndex.Columns))
				}
				lastIndex.Columns = append(lastIndex.Columns, columnName)
			}
		}

		return errs.Wrap(rows.Err())
	}()
	if err != nil {
		return nil, err
	}

	schema.Sort()
	return schema, nil
}

func translateType(spannerType string) (normalType string) {
	switch spannerType {
	case "BOOL":
		return "boolean"
	case "INT64":
		return "bigint"
	case "FLOAT64":
		return "double precision"
	case "STRING(MAX)":
		return "text"
	case "STRING(36)":
		return "uuid"
	case "TIMESTAMP":
		return "timestamp with time zone"
	case "DATE":
		return "date"
	case "BYTES(MAX)":
		return "bytea"
	case "ARRAY<BOOL>":
		return "boolean[]"
	case "ARRAY<INT64>":
		return "bigint[]"
	case "ARRAY<FLOAT64>":
		return "double precision[]"
	case "ARRAY<STRING(MAX)>":
		return "text[]"
	case "ARRAY<TIMESTAMP>":
		return "timestamp with time zone[]"
	case "ARRAY<DATE>":
		return "date[]"
	case "ARRAY<BYTES(MAX)>":
		return "bytea[]"
	default:
		return strings.ToLower(spannerType)
	}
}

var timestampSecondsRx = regexp.MustCompile(`timestamp_seconds\((\d+)\)`)

func parseColumnDefault(columnDefault string) string {
	if columnDefault == "current_timestamp" {
		columnDefault = strings.ToUpper(columnDefault)
	}
	if match := valueTypePrefixRx.FindStringSubmatch(columnDefault); match != nil {
		// throw away type annotation
		columnDefault = match[2]
		if match[1] == "JSON" && columnDefault[0] == '"' && columnDefault[len(columnDefault)-1] == '"' {
			columnDefault = "'" + columnDefault[1:len(columnDefault)-1] + "'"
		}
	}
	if match := sequenceRx.FindStringSubmatch(columnDefault); match != nil {
		columnDefault = "nextval('" + match[1] + "_seq')"
	}
	if match := timestampSecondsRx.FindStringSubmatch(columnDefault); match != nil {
		seconds, err := strconv.ParseInt(match[1], 10, 64)
		if err == nil {
			columnDefault = time.Unix(seconds, 0).UTC().Format("2006-01-02 15:04:05.000000-07")
		}
	}
	return columnDefault
}
