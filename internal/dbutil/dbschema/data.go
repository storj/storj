// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

import "sort"

// Data is the database content formatted as strings
type Data struct {
	Tables []*TableData
}

// TableData is content of a sql table
type TableData struct {
	Name    string
	Columns []string
	Rows    []RowData
}

// RowData is content of a single row
type RowData []string

// AddTable adds a new table.
func (data *Data) AddTable(table *TableData) {
	data.Tables = append(data.Tables, table)
}

// AddRow adds a new row.
func (table *TableData) AddRow(row RowData) {
	table.Rows = append(table.Rows, row)
}

// Sort sorts all tables.
func (data *Data) Sort() {
	for _, table := range data.Tables {
		table.Sort()
	}
}

// Sort sorts all rows.
func (table *TableData) Sort() {
	sort.Slice(table.Rows, func(i, k int) bool {
		return lessStrings(table.Rows[i], table.Rows[k])
	})
}
