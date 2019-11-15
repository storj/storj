// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package dbschema package implements querying and comparing schemas for testing.
package dbschema

import (
	"database/sql"
	"sort"
)

// Queryer is a representation for something that can query.
type Queryer interface {
	// Query executes a query that returns rows, typically a SELECT.
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// Schema is the database structure.
type Schema struct {
	Tables  []*Table
	Indexes []*Index
}

// Table is a sql table.
type Table struct {
	Name       string
	Columns    []*Column
	PrimaryKey []string
	Unique     [][]string
}

// Column is a sql column.
type Column struct {
	Name       string
	Type       string
	IsNullable bool
	Reference  *Reference
}

// Reference is a column foreign key.
type Reference struct {
	Table    string
	Column   string
	OnDelete string
	OnUpdate string
}

// Index is an index for a table.
type Index struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
	Partial string // partial expression
}

// EnsureTable returns the table with the specified name and creates one if needed.
func (schema *Schema) EnsureTable(tableName string) *Table {
	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table
		}
	}
	table := &Table{Name: tableName}
	schema.Tables = append(schema.Tables, table)
	return table
}

// DropTable removes the specified table
func (schema *Schema) DropTable(tableName string) {
	for i, table := range schema.Tables {
		if table.Name == tableName {
			schema.Tables = append(schema.Tables[:i], schema.Tables[i+1:]...)
			return
		}
	}
}

// AddColumn adds the column to the table.
func (table *Table) AddColumn(column *Column) {
	table.Columns = append(table.Columns, column)
}

// FindColumn finds a column in the table
func (table *Table) FindColumn(columnName string) (*Column, bool) {
	for _, column := range table.Columns {
		if column.Name == columnName {
			return column, true
		}
	}
	return nil, false
}

// ColumnNames returns column names
func (table *Table) ColumnNames() []string {
	columns := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		columns[i] = column.Name
	}
	return columns
}

// Sort sorts tables and indexes
func (schema *Schema) Sort() {
	sort.Slice(schema.Tables, func(i, k int) bool {
		return schema.Tables[i].Name < schema.Tables[k].Name
	})
	for _, table := range schema.Tables {
		table.Sort()
	}
	sort.Slice(schema.Indexes, func(i, k int) bool {
		return schema.Indexes[i].Name < schema.Indexes[k].Name
	})
}

// Sort sorts columns, primary keys and unique
func (table *Table) Sort() {
	sort.Slice(table.Columns, func(i, k int) bool {
		return table.Columns[i].Name < table.Columns[k].Name
	})

	sort.Strings(table.PrimaryKey)
	for i := range table.Unique {
		sort.Strings(table.Unique[i])
	}

	sort.Slice(table.Unique, func(i, k int) bool {
		return lessStrings(table.Unique[i], table.Unique[k])
	})
}

func lessStrings(a, b []string) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for k := 0; k < n; k++ {
		if a[k] < b[k] {
			return true
		} else if a[k] > b[k] {
			return false
		}
	}
	return len(a) < len(b)
}
