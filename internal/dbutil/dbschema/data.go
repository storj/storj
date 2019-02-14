// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

import (
	"sort"
	"strings"

	"github.com/zeebo/errs"
)

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

// FindTable finds a table by name
func (data *Data) FindTable(tableName string) (*TableData, bool) {
	for _, table := range data.Tables {
		if table.Name == tableName {
			return table, true
		}
	}
	return nil, false
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

// Clone returns a clone of row data.
func (row RowData) Clone() RowData {
	return append(RowData{}, row...)
}

// QueryData loads all data from tables
func QueryData(db Queryer, schema *Schema, quoteColumn func(string) string) (*Data, error) {
	data := &Data{}

	for _, tableSchema := range schema.Tables {
		columnNames := tableSchema.ColumnNames()
		table := &TableData{
			Name:    tableSchema.Name,
			Columns: columnNames,
		}

		// quote column names
		quotedColumns := make([]string, len(columnNames))
		for i, columnName := range columnNames {
			quotedColumns[i] = quoteColumn(columnName)
		}

		// build query for selecting all values
		query := `SELECT ` + strings.Join(quotedColumns, ", ") + ` FROM ` + table.Name

		err := func() (err error) {
			rows, err := db.Query(query)
			if err != nil {
				return err
			}
			defer func() { err = errs.Combine(err, rows.Close()) }()

			row := make(RowData, len(columnNames))
			rowargs := make([]interface{}, len(columnNames))
			for i := range row {
				rowargs[i] = &row[i]
			}

			for rows.Next() {
				err := rows.Scan(rowargs...)
				if err != nil {
					return err
				}

				table.AddRow(row.Clone())
			}

			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}
