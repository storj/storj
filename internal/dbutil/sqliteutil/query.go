// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"database/sql"
	"regexp"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/dbschema"
)

type definition struct {
	name string
	sql  string
}

// QuerySchema loads the schema from sqlite database.
func QuerySchema(db dbschema.Queryer) (*dbschema.Schema, error) {
	schema := &dbschema.Schema{}

	tableDefinitions := make([]*definition, 0)
	indexDefinitions := make([]*definition, 0)

	// find tables and indexes
	err := func() error {
		rows, err := db.Query(`
			SELECT name, type, sql FROM sqlite_master WHERE sql NOT NULL AND name NOT LIKE 'sqlite_%'
		`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, rows.Close()) }()

		for rows.Next() {
			var defName, defType, defSQL string
			err := rows.Scan(&defName, &defType, &defSQL)
			if err != nil {
				return errs.Wrap(err)
			}
			if defType == "table" {
				tableDefinitions = append(tableDefinitions, &definition{name: defName, sql: defSQL})
			} else if defType == "index" {
				indexDefinitions = append(indexDefinitions, &definition{name: defName, sql: defSQL})
			}
		}

		return rows.Err()
	}()
	if err != nil {
		return nil, err
	}

	err = discoverTables(db, schema, tableDefinitions)
	if err != nil {
		return nil, err
	}

	err = discoverIndexes(db, schema, indexDefinitions)
	if err != nil {
		return nil, err
	}

	schema.Sort()
	return schema, nil
}

func discoverTables(db dbschema.Queryer, schema *dbschema.Schema, tableDefinitions []*definition) (err error) {
	for _, definition := range tableDefinitions {
		table := schema.EnsureTable(definition.name)

		tableRows, err := db.Query(`PRAGMA table_info(` + definition.name + `)`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, tableRows.Close()) }()

		for tableRows.Next() {
			var defaultValue sql.NullString
			var index, name, columnType string
			var pk int
			var notNull bool
			err := tableRows.Scan(&index, &name, &columnType, &notNull, &defaultValue, &pk)
			if err != nil {
				return errs.Wrap(err)
			}

			column := &dbschema.Column{
				Name:       name,
				Type:       columnType,
				IsNullable: !notNull && pk == 0,
			}
			table.AddColumn(column)
			if pk > 0 {
				if table.PrimaryKey == nil {
					table.PrimaryKey = make([]string, 0)
				}
				table.PrimaryKey = append(table.PrimaryKey, name)
			}

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

		keysRows, err := db.Query(`PRAGMA foreign_key_list(` + definition.name + `)`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, keysRows.Close()) }()

		for keysRows.Next() {
			var id, sec int
			var tableName, from, to, onUpdate, onDelete, match string
			err := keysRows.Scan(&id, &sec, &tableName, &from, &to, &onUpdate, &onDelete, &match)
			if err != nil {
				return errs.Wrap(err)
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
	}
	return errs.Wrap(err)
}

func discoverIndexes(db dbschema.Queryer, schema *dbschema.Schema, indexDefinitions []*definition) (err error) {
	// TODO improve indexes discovery
	for _, definition := range indexDefinitions {
		index := &dbschema.Index{
			Name: definition.name,
		}
		schema.Indexes = append(schema.Indexes, index)

		indexRows, err := db.Query(`PRAGMA index_info(` + definition.name + `)`)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { err = errs.Combine(err, indexRows.Close()) }()

		for indexRows.Next() {
			var name *string
			var seqno, cid int
			err := indexRows.Scan(&seqno, &cid, &name)
			if err != nil {
				return errs.Wrap(err)
			}
			if name != nil {
				index.Columns = append(index.Columns, *name)
			} else {
				matches := rxIndexExpr.FindStringSubmatch(definition.sql)
				index.Columns = append(index.Columns, matches[1])
			}
		}

		matches := rxIndexTable.FindStringSubmatch(definition.sql)
		index.Table = strings.TrimSpace(matches[1])

		if matches := rxIndexPartial.FindStringSubmatch(definition.sql); len(matches) > 0 {
			index.Partial = strings.TrimSpace(matches[1])
		}
	}
	return errs.Wrap(err)
}

var (
	// matches UNIQUE (a,b)
	rxUnique = regexp.MustCompile(`UNIQUE\s*\((.*?)\)`)

	// matches ON (a,b)
	rxIndexTable = regexp.MustCompile(`ON\s*([^(]*)\(`)

	// matches ON table(expr)
	rxIndexExpr = regexp.MustCompile(`ON\s*[^(]*\((.*)\)`)

	// matches WHERE (partial expression)
	rxIndexPartial = regexp.MustCompile(`WHERE (.*)$`)
)
