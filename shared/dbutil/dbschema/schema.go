// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package dbschema package implements querying and comparing schemas for testing.
package dbschema

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"storj.io/storj/shared/tagsql"
)

// Queryer is a representation for something that can query.
type Queryer interface {
	// QueryRowContext executes a query that returns a single row.
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	// QueryContext executes a query that returns rows, typically a SELECT.
	QueryContext(ctx context.Context, query string, args ...interface{}) (tagsql.Rows, error)
}

// Schema is the database structure.
type Schema struct {
	Tables    []*Table
	Indexes   []*Index
	Sequences []string
}

func (schema Schema) String() string {
	var tables []string
	for _, table := range schema.Tables {
		tables = append(tables, table.String())
	}

	var indexes []string
	for _, index := range schema.Indexes {
		indexes = append(indexes, index.String())
	}

	return fmt.Sprintf("Tables:\n\t%s\nIndexes:\n\t%s\nSequences:\n\t%s\n",
		indent(strings.Join(tables, "\n")),
		indent(strings.Join(indexes, "\n")),
		indent(strings.Join(schema.Sequences, "\n")),
	)
}

// Table is a sql table.
type Table struct {
	Name       string
	Columns    []*Column
	PrimaryKey []string
	Unique     [][]string
	Checks     []string
}

func (table Table) String() string {
	var columns []string
	for _, column := range table.Columns {
		columns = append(columns, column.String())
	}

	var uniques []string
	for _, unique := range table.Unique {
		uniques = append(uniques, strings.Join(unique, " "))
	}

	return fmt.Sprintf("Name: %s\nColumns:\n\t%s\nPrimaryKey: %s\nUniques:\n\t%s\nChecks:\n\t\t%s\n",
		table.Name,
		indent(strings.Join(columns, "\n")),
		strings.Join(table.PrimaryKey, " "),
		indent(strings.Join(uniques, "\n")),
		indent(strings.Join(table.Checks, "\n")),
	)
}

// Column is a sql column.
type Column struct {
	Name       string
	Type       string
	IsNullable bool
	Default    string
	Reference  *Reference
}

func (column Column) String() string {
	return fmt.Sprintf("Name: %s\nType: %s\nNullable: %t\nDefault: %q\nReference: %s",
		column.Name,
		column.Type,
		column.IsNullable,
		column.Default,
		column.Reference)
}

// Reference is a column foreign key.
type Reference struct {
	Table    string
	Column   string
	OnDelete string
	OnUpdate string
}

func (reference *Reference) String() string {
	if reference == nil {
		return "nil"
	}
	return fmt.Sprintf("Reference<Table: %s, Column: %s, OnDelete: %s, OnUpdate: %s>",
		reference.Table,
		reference.Column,
		reference.OnDelete,
		reference.OnUpdate)
}

// Index is an index for a table.
type Index struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
	Partial string // partial expression
}

func (index Index) String() string {
	return fmt.Sprintf("Index<Table: %s, Name: %s, Columns: %s, Unique: %t, Partial: %q>",
		index.Table,
		index.Name,
		indent(strings.Join(index.Columns, " ")),
		index.Unique,
		index.Partial)
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

// DropTable removes the specified table.
func (schema *Schema) DropTable(tableName string) {
	for i, table := range schema.Tables {
		if table.Name == tableName {
			schema.Tables = append(schema.Tables[:i], schema.Tables[i+1:]...)
			break
		}
	}

	j := 0
	for _, index := range schema.Indexes {
		if index.Table == tableName {
			continue
		}
		schema.Indexes[j] = index
		j++
	}
	schema.Indexes = schema.Indexes[:j:j]
}

// FindTable returns the specified table.
func (schema *Schema) FindTable(tableName string) (*Table, bool) {
	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return nil, false
}

// FindIndex finds index in the schema.
func (schema *Schema) FindIndex(name string) (*Index, bool) {
	for _, idx := range schema.Indexes {
		if idx.Name == name {
			return idx, true
		}
	}

	return nil, false
}

// DropIndex removes the specified index.
func (schema *Schema) DropIndex(name string) {
	for i, idx := range schema.Indexes {
		if idx.Name == name {
			schema.Indexes = append(schema.Indexes[:i], schema.Indexes[i+1:]...)
			return
		}
	}
}

// AddColumn adds the column to the table.
func (table *Table) AddColumn(column *Column) {
	table.Columns = append(table.Columns, column)
}

// RemoveColumn removes the column from the table.
func (table *Table) RemoveColumn(columnName string) {
	for i, column := range table.Columns {
		if column.Name == columnName {
			table.Columns = append(table.Columns[:i], table.Columns[i+1:]...)
			return
		}
	}
}

// FindColumn finds a column in the table.
func (table *Table) FindColumn(columnName string) (*Column, bool) {
	for _, column := range table.Columns {
		if column.Name == columnName {
			return column, true
		}
	}
	return nil, false
}

// ColumnNames returns column names.
func (table *Table) ColumnNames() []string {
	columns := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		columns[i] = column.Name
	}
	return columns
}

// Sort sorts tables and indexes.
func (schema *Schema) Sort() {
	sort.Slice(schema.Tables, func(i, k int) bool {
		return schema.Tables[i].Name < schema.Tables[k].Name
	})
	for _, table := range schema.Tables {
		table.Sort()
	}
	sort.Slice(schema.Indexes, func(i, k int) bool {
		switch {
		case schema.Indexes[i].Table < schema.Indexes[k].Table:
			return true
		case schema.Indexes[i].Table > schema.Indexes[k].Table:
			return false
		default:
			return schema.Indexes[i].Name < schema.Indexes[k].Name
		}
	})
}

// HasSequence returns with true if sequence is added to the scheme.
func (schema Schema) HasSequence(name string) bool {
	for _, seq := range schema.Sequences {
		if seq == name {
			return true
		}
	}
	return false
}

// Sort sorts columns, primary keys and unique.
func (table *Table) Sort() {
	sort.Slice(table.Columns, func(i, k int) bool {
		return table.Columns[i].Name < table.Columns[k].Name
	})

	sort.Strings(table.PrimaryKey)

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

func indent(lines string) string {
	return strings.TrimSpace(strings.ReplaceAll(lines, "\n", "\n\t"))
}
