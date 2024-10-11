// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbschema"
)

type definition struct {
	name  string
	table string
	sql   string
}

// QuerySchema loads the schema from sqlite database.
func QuerySchema(ctx context.Context, db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	tableDefinitions := make([]*definition, 0)
	indexDefinitions := make([]*definition, 0)

	// find tables and indexes
	err := func() error {
		rows, err := db.QueryContext(ctx, `
			SELECT name, tbl_name, type, sql FROM sqlite_master WHERE sql NOT NULL AND name NOT LIKE 'sqlite_%'
		`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var defName, defTblName, defType, defSQL string
			err := rows.Scan(&defName, &defTblName, &defType, &defSQL)
			if err != nil {
				return errs.Wrap(err)
			}
			if defType == "table" {
				tableDefinitions = append(tableDefinitions, &definition{name: defName, sql: defSQL})
			} else if defType == "index" {
				indexDefinitions = append(indexDefinitions, &definition{name: defName, sql: defSQL, table: defTblName})
			}
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	err = discoverTables(ctx, db, schema, tableDefinitions)
	if err != nil {
		return nil, err
	}

	err = discoverIndexes(ctx, db, schema, indexDefinitions)
	if err != nil {
		return nil, err
	}

	schema.Sort()
	return schema, nil
}

func discoverTables(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema, tableDefinitions []*definition) (err error) {
	for _, definition := range tableDefinitions {
		if err := discoverTable(ctx, db, schema, definition); err != nil {
			return err
		}
	}
	return errs.Wrap(err)
}

func discoverTable(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema, definition *definition) (err error) {
	table := schema.EnsureTable(definition.name)

	tableRows, err := db.QueryContext(ctx, `PRAGMA table_info(`+definition.name+`)`)
	if err != nil {
		return errs.Wrap(err)
	}

	for tableRows.Next() {
		var defaultValue sql.NullString
		var index, name, columnType string
		var pk int
		var notNull bool
		err := tableRows.Scan(&index, &name, &columnType, &notNull, &defaultValue, &pk)
		if err != nil {
			return errs.Wrap(errs.Combine(tableRows.Err(), tableRows.Close(), err))
		}

		column := &dbschema.Column{
			Name:       name,
			Type:       columnType,
			IsNullable: !notNull && pk == 0,
		}
		table.AddColumn(column)
		if pk > 0 {
			table.PrimaryKey = append(table.PrimaryKey, name)
		}
	}
	err = errs.Combine(tableRows.Err(), tableRows.Close())
	if err != nil {
		return errs.Wrap(err)
	}

	matches := rxUnique.FindAllStringSubmatch(definition.sql, -1)
	for _, match := range matches {
		// TODO feel this can be done easier
		var columns []string
		for _, name := range strings.Split(match[1], ",") {
			columns = append(columns, strings.TrimSpace(name))
		}

		table.Unique = append(table.Unique, columns)
	}

	keysRows, err := db.QueryContext(ctx, `PRAGMA foreign_key_list(`+definition.name+`)`)
	if err != nil {
		return errs.Wrap(err)
	}

	for keysRows.Next() {
		var id, sec int
		var tableName, from, to, onUpdate, onDelete, match string
		err := keysRows.Scan(&id, &sec, &tableName, &from, &to, &onUpdate, &onDelete, &match)
		if err != nil {
			return errs.Wrap(errs.Combine(keysRows.Err(), keysRows.Close(), err))
		}

		column, found := table.FindColumn(from)
		if found {
			if onDelete == "NO ACTION" {
				onDelete = ""
			}
			if onUpdate == "NO ACTION" {
				onUpdate = ""
			}
			column.Reference = &dbschema.Reference{
				Table:    tableName,
				Column:   to,
				OnUpdate: onUpdate,
				OnDelete: onDelete,
			}
		}
	}
	err = errs.Combine(keysRows.Err(), keysRows.Close())
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func discoverIndexes(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema, indexDefinitions []*definition) (err error) {
	// TODO improve indexes discovery
	for _, definition := range indexDefinitions {
		index := &dbschema.Index{
			Name:  definition.name,
			Table: definition.table,
		}

		schema.Indexes = append(schema.Indexes, index)

		indexRows, err := db.QueryContext(ctx, `PRAGMA index_info(`+definition.name+`)`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, indexRows.Close()) }()

		type indexInfo struct {
			name  *string
			seqno int
			cid   int
		}

		var indexInfos []indexInfo
		for indexRows.Next() {
			var info indexInfo
			err := indexRows.Scan(&info.seqno, &info.cid, &info.name)
			if err != nil {
				return errs.Wrap(err)
			}
			indexInfos = append(indexInfos, info)
		}

		sort.SliceStable(indexInfos, func(i, j int) bool {
			return indexInfos[i].seqno < indexInfos[j].seqno
		})

		sqlDef := definition.sql

		var parsedColumns []string
		parseColumns := func() []string {
			if parsedColumns != nil {
				return parsedColumns
			}

			var base string
			if matches := rxIndexExpr.FindStringSubmatchIndex(strings.ToUpper(sqlDef)); len(matches) > 0 {
				base = sqlDef[matches[2]:matches[3]]
			}

			parsedColumns = strings.Split(base, ",")
			return parsedColumns
		}

		for _, info := range indexInfos {
			if info.name != nil {
				index.Columns = append(index.Columns, *info.name)
				continue
			}

			if info.cid == -1 {
				index.Columns = append(index.Columns, "rowid")
			} else if info.cid == -2 {
				index.Columns = append(index.Columns, parseColumns()[info.seqno])
			}
		}

		// unique
		if strings.Contains(definition.sql, "CREATE UNIQUE INDEX") {
			index.Unique = true
		}
		// partial
		if matches := rxIndexPartial.FindStringSubmatch(definition.sql); len(matches) > 0 {
			index.Partial = strings.TrimSpace(matches[1])
		}
	}
	return errs.Wrap(err)
}

var (
	// matches "UNIQUE (a,b)".
	rxUnique = regexp.MustCompile(`UNIQUE\s*\((.*?)\)`)

	// matches "ON table(expr)".
	rxIndexExpr = regexp.MustCompile(`ON\s*[^(]*\((.*)\)`)

	// matches "WHERE (partial expression)".
	rxIndexPartial = regexp.MustCompile(`WHERE (.*)$`)
)
