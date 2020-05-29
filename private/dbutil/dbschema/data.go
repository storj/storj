// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

import (
	"context"
	"fmt"
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

// ColumnData is a value of a column within a row.
type ColumnData struct {
	Column string
	Value  string
}

// String returns a string representation of the column.
func (c ColumnData) String() string {
	return fmt.Sprintf("%s:%s", c.Column, c.Value)
}

// RowData is content of a single row
type RowData []ColumnData

// Less returns true if one row is less than the other.
func (row RowData) Less(b RowData) bool {
	n := len(row)
	if len(b) < n {
		n = len(b)
	}
	for k := 0; k < n; k++ {
		if row[k].Value < b[k].Value {
			return true
		} else if row[k].Value > b[k].Value {
			return false
		}
	}
	return len(row) < len(b)
}

// AddTable adds a new table.
func (data *Data) AddTable(table *TableData) {
	data.Tables = append(data.Tables, table)
}

// AddRow adds a new row.
func (table *TableData) AddRow(row RowData) error {
	if len(row) != len(table.Columns) {
		return errs.New("inconsistent row added to table")
	}
	for i, cdata := range row {
		if cdata.Column != table.Columns[i] {
			return errs.New("inconsistent row added to table")
		}
	}
	table.Rows = append(table.Rows, row)
	return nil
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
		return table.Rows[i].Less(table.Rows[k])
	})
}

// Clone returns a clone of row data.
func (row RowData) Clone() RowData {
	return append(RowData{}, row...)
}

// QueryData loads all data from tables
func QueryData(ctx context.Context, db Queryer, schema *Schema, quoteColumn func(string) string) (*Data, error) {
	data := &Data{}

	for _, tableSchema := range schema.Tables {
		columnNames := tableSchema.ColumnNames()
		table := &TableData{
			Name:    tableSchema.Name,
			Columns: columnNames,
		}
		data.AddTable(table)

		// quote column names
		quotedColumns := make([]string, len(columnNames))
		for i, columnName := range columnNames {
			quotedColumns[i] = quoteColumn(columnName)
		}

		// build query for selecting all values
		query := `SELECT ` + strings.Join(quotedColumns, ", ") + ` FROM ` + table.Name

		err := func() (err error) {
			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return err
			}
			defer func() { err = errs.Combine(err, rows.Close()) }()

			row := make(RowData, len(columnNames))
			rowargs := make([]interface{}, len(columnNames))
			for i := range row {
				row[i].Column = columnNames[i]
				rowargs[i] = &row[i].Value
			}

			for rows.Next() {
				err := rows.Scan(rowargs...)
				if err != nil {
					return err
				}

				if err := table.AddRow(row.Clone()); err != nil {
					return err
				}
			}

			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	data.Sort()
	return data, nil
}
