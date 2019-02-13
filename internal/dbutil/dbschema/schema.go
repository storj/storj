// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// dbschema package implements querying and comparing schemas for testing.
package dbschema

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
