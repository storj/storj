// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/shared/dbutil/dbschema"
)

func TestValidateColumnName(t *testing.T) {
	tcases := []struct {
		desc   string
		column string
		isErr  bool
	}{
		{desc: "valid column: all lowercase letters", column: "acolumn", isErr: false},
		{desc: "valid column: all uppercase letters", column: "ACOLUMN", isErr: false},
		{desc: "valid column: all lower and upper case letters", column: "aColumn", isErr: false},
		{desc: "valid column: all letters and numbers", column: "1Column", isErr: false},
		{desc: "valid column: with underscores", column: "a_column_2", isErr: false},
		{desc: "valid column: with dashes", column: "a-col_umn-2", isErr: false},
		{desc: "valid column: single lowercase letter", column: "e", isErr: false},
		{desc: "valid column: single uppercase letter", column: "Z", isErr: false},
		{desc: "valid column: single number", column: "7", isErr: false},
		{desc: "valid column: single underscore", column: "_", isErr: false},
		{desc: "invalid column: beginning with dash", column: "-col_umn2", isErr: true},
		{desc: "invalid column: ending with dash", column: "Column-", isErr: true},
		{desc: "invalid column: 2 dashes in a row", column: "Col--umn2", isErr: true},
		{desc: "invalid column: containing forbidden chars (?)", column: "Col?umn", isErr: true},
		{desc: "invalid column: containing forbidden chars (*)", column: "Column*", isErr: true},
		{desc: "invalid column: containing forbidden chars (blank space)", column: "a Column", isErr: true},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			err := dbschema.ValidateColumnName(tc.column)
			isErr := err != nil
			assert.Equal(t, tc.isErr, isErr, "returns error")
		})
	}
}

func TestValidateTableName(t *testing.T) {
	tcases := []struct {
		desc  string
		table string
		isErr bool
	}{
		{desc: "valid table: all lowercase letters", table: "atable", isErr: false},
		{desc: "valid table: all uppercase letters", table: "ATABLE", isErr: false},
		{desc: "valid table: all lower and upper case letters", table: "aTable", isErr: false},
		{desc: "valid table: all letters and numbers", table: "1Table", isErr: false},
		{desc: "valid table: with underscores", table: "a_table_2", isErr: false},
		{desc: "valid table: with dashes", table: "a-tab_le-2", isErr: false},
		{desc: "valid table: single lowercase letter", table: "e", isErr: false},
		{desc: "valid table: single uppercase letter", table: "Z", isErr: false},
		{desc: "valid table: table with schema", table: "a.Table", isErr: false},
		{desc: "valid table: single number", table: "7", isErr: false},
		{desc: "valid table: single underscore", table: "_", isErr: false},
		{desc: "invalid table: beginning with dash", table: "-tab_le2", isErr: true},
		{desc: "invalid table: ending with dash", table: "Table-", isErr: true},
		{desc: "invalid table: 2 dashes in a row", table: "Table--2", isErr: true},
		{desc: "invalid table: containing forbidden chars (?)", table: "Tab?e", isErr: true},
		{desc: "invalid table: containing forbidden chars (*)", table: "*table", isErr: true},
		{desc: "invalid table: containing forbidden chars (blank space)", table: "a Table", isErr: true},
		{desc: "invalid table: more than one dot)", table: "public.t1.t2", isErr: true},
	}

	for _, tc := range tcases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			err := dbschema.ValidateTableName(tc.table)
			isErr := err != nil
			assert.Equal(t, tc.isErr, isErr, "returns error")
		})
	}
}
