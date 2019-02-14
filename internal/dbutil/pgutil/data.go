// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/dbschema"
)

// QueryData loads all data from tables
func QueryData(db Queryer, schema *dbschema.Schema) (*dbschema.Data, error) {
	data := &dbschema.Data{}

	for _, tableSchema := range schema.Tables {
		columnNames := tableSchema.ColumnNames()
		table := &dbschema.TableData{
			Name:    tableSchema.Name,
			Columns: columnNames,
		}

		query := `SELECT ` + strings.Join(quoteColumns(columnNames), ", ") + ` FROM ` + table.Name

		err := func() (err error) {
			rows, err := db.Query(query)
			if err != nil {
				return err
			}
			defer func() { err = errs.Combine(err, rows.Close()) }()

			row := make(dbschema.RowData, len(columnNames))
			rowargs := make([]interface{}, len(columnNames))
			for i := range row {
				rowargs[i] = &row[i]
			}

			for rows.Next() {
				err := rows.Scan(rowargs...)
				if err != nil {
					return err
				}

				table.AddRow(cloneRow(row))
			}

			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func cloneRow(row dbschema.RowData) dbschema.RowData {
	return append(dbschema.RowData{}, row...)
}

func quoteColumns(columnNames []string) []string {
	columns := make([]string, len(columnNames))
	for i, columnName := range columnNames {
		cn := strconv.Quote(columnName)
		columns[i] = `quote_nullable(` + cn + `) as ` + cn
	}
	return columns
}
