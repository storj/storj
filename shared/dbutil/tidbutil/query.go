// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"sort"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbschema"
)

// QuerySnapshot loads the schema and data snapshot from a TiDB database.
//
// Mirrors pgutil.QuerySnapshot for the metabase TiDB backend so that
// TestMigration can compare a "prod" migration to a "test" migration
// without depending on Postgres-only features.
func QuerySnapshot(ctx context.Context, db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(ctx, db)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	data, err := QueryData(ctx, db, schema)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, nil
}

// QuerySchema reads the current schema (tables, columns, primary keys,
// uniques, indexes) from the active TiDB database.
func QuerySchema(ctx context.Context, db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	// Collect columns. TiDB exposes information_schema with the same shape
	// as MySQL.
	{
		rows, err := db.QueryContext(ctx, `
			SELECT TABLE_NAME, COLUMN_NAME, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, ''), DATA_TYPE
			FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE()
			ORDER BY TABLE_NAME, ORDINAL_POSITION
		`)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = func() (err error) {
			defer func() { err = errs.Combine(err, rows.Close()) }()
			for rows.Next() {
				var tableName, columnName, isNullable, columnDefault, dataType string
				if err := rows.Scan(&tableName, &columnName, &isNullable, &columnDefault, &dataType); err != nil {
					return errs.Wrap(err)
				}
				table := schema.EnsureTable(tableName)
				table.AddColumn(&dbschema.Column{
					Name:       columnName,
					Type:       dataType,
					IsNullable: isNullable == "YES",
					Default:    columnDefault,
				})
			}
			return errs.Wrap(rows.Err())
		}()
		if err != nil {
			return nil, err
		}
	}

	// Collect indexes (including primary keys and uniques).
	type indexRow struct {
		table     string
		indexName string
		nonUnique int
		seq       int
		column    string
	}
	var rows []indexRow
	{
		r, err := db.QueryContext(ctx, `
			SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME
			FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX
		`)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = func() (err error) {
			defer func() { err = errs.Combine(err, r.Close()) }()
			for r.Next() {
				var ir indexRow
				if err := r.Scan(&ir.table, &ir.indexName, &ir.nonUnique, &ir.seq, &ir.column); err != nil {
					return errs.Wrap(err)
				}
				rows = append(rows, ir)
			}
			return errs.Wrap(r.Err())
		}()
		if err != nil {
			return nil, err
		}
	}

	// Group statistics rows into indexes.
	type indexKey struct{ table, name string }
	indexed := map[indexKey]*dbschema.Index{}
	for _, ir := range rows {
		key := indexKey{ir.table, ir.indexName}
		idx, ok := indexed[key]
		if !ok {
			idx = &dbschema.Index{
				Name:   ir.indexName,
				Table:  ir.table,
				Unique: ir.nonUnique == 0,
			}
			indexed[key] = idx
		}
		idx.Columns = append(idx.Columns, ir.column)
	}

	// Promote PRIMARY index to PrimaryKey on the table; treat other unique
	// indexes as both an Index entry and a Table.Unique entry, mirroring
	// what the Postgres path does.
	for key, idx := range indexed {
		table := schema.EnsureTable(key.table)
		if idx.Name == "PRIMARY" {
			table.PrimaryKey = append([]string(nil), idx.Columns...)
			continue
		}
		if idx.Unique {
			table.Unique = append(table.Unique, append([]string(nil), idx.Columns...))
		}
		schema.Indexes = append(schema.Indexes, idx)
	}

	// Collect FK constraints. TiDB tolerates them syntactically but most of
	// our schemas don't use them; this still keeps QuerySchema useful for
	// other callers.
	{
		r, err := db.QueryContext(ctx, `
			SELECT
				rc.CONSTRAINT_NAME,
				kcu.TABLE_NAME,
				kcu.COLUMN_NAME,
				kcu.REFERENCED_TABLE_NAME,
				kcu.REFERENCED_COLUMN_NAME,
				rc.UPDATE_RULE,
				rc.DELETE_RULE,
				kcu.ORDINAL_POSITION
			FROM information_schema.REFERENTIAL_CONSTRAINTS rc
			JOIN information_schema.KEY_COLUMN_USAGE kcu
			  ON kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
			 AND kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			WHERE rc.CONSTRAINT_SCHEMA = DATABASE()
			ORDER BY rc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
		`)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = func() (err error) {
			defer func() { err = errs.Combine(err, r.Close()) }()
			fks := map[string]*dbschema.ForeignKey{}
			fkOrder := []string{}
			fkTable := map[string]string{}
			for r.Next() {
				var name, tname, col, refTable, refCol, onUpdate, onDelete string
				var ord int
				if err := r.Scan(&name, &tname, &col, &refTable, &refCol, &onUpdate, &onDelete, &ord); err != nil {
					return errs.Wrap(err)
				}
				fk, ok := fks[name]
				if !ok {
					fk = &dbschema.ForeignKey{
						Name:         name,
						ForeignTable: refTable,
						OnUpdate:     onUpdate,
						OnDelete:     onDelete,
					}
					fks[name] = fk
					fkOrder = append(fkOrder, name)
					fkTable[name] = tname
				}
				fk.LocalColumns = append(fk.LocalColumns, col)
				fk.ForeignColumns = append(fk.ForeignColumns, refCol)
			}
			if err := r.Err(); err != nil {
				return errs.Wrap(err)
			}
			for _, name := range fkOrder {
				table := schema.EnsureTable(fkTable[name])
				table.ForeignKeys = append(table.ForeignKeys, fks[name])
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	// TiDB has no SQL-level sequences: AUTO_INCREMENT is per-column.
	sort.Strings(schema.Sequences)
	schema.Sort()
	return schema, nil
}

// QueryData mirrors pgutil.QueryData using TiDB-quoted identifiers.
func QueryData(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema) (*dbschema.Data, error) {
	return dbschema.QueryData(ctx, db, schema, func(columnName string) string {
		quoted := QuoteIdentifier(columnName)
		// COALESCE(...) preserves NULL distinction by emitting an
		// unquoted "NULL" sentinel like quote_nullable() does in Postgres.
		return `IFNULL(QUOTE(` + quoted + `), 'NULL') AS ` + quoted
	})
}
