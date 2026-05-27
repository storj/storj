// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	type simple struct {
		Field string `csv:"field"`
	}
	type notag struct {
		Field string
	}
	type unmapped struct {
		Field    string `csv:"field"`
		Unmapped string `csv:"unmapped"`
	}
	type ignored struct {
		Field   string `csv:"field"`
		Ignored string `csv:"-"`
	}
	type embedded struct {
		Field simple `csv:"field"`
	}

	simplePtrPtr := func(s simple) **simple {
		ptr := &s
		return &ptr
	}

	for _, tt := range []struct {
		name string
		csv  string
		obj  any
		out  any
		err  string
	}{
		{
			name: "normal fields",
			csv:  normalFieldsCSV,
			obj:  &NormalFields{},
			out:  &normalFields,
		},
		{
			name: "optional fields unset",
			csv:  optionalFieldsUnsetCSV,
			obj:  &OptionalFields{},
			out:  &optionalFieldsUnset,
		},
		{
			name: "optional fields set",
			csv:  optionalFieldsSetCSV,
			obj:  &OptionalFields{},
			out:  &optionalFieldsSet,
		},
		{
			name: "defined fields",
			csv:  definedFieldsCSV,
			obj:  &DefinedFields{},
			out:  &definedFields,
		},
		{
			name: "slice of struct",
			csv:  normalFieldsCSV,
			obj:  &[]NormalFields{},
			out:  &[]NormalFields{normalFields},
		},
		{
			name: "nil pointer to struct",
			csv:  "field\nvalue\n",
			obj:  simplePtrPtr(simple{}),
			out:  simplePtrPtr(simple{Field: "value"}),
		},
		{
			name: "slice of pointer to struct",
			csv:  normalFieldsCSV,
			obj:  &[]*NormalFields{},
			out:  &[]*NormalFields{&normalFields},
		},
		{
			name: "nil",
			csv:  "field\nvalue",
			obj:  nil,
			err:  "strictcsv: destination (<nil>) cannot be nil",
		},
		{
			name: "nil pointer",
			csv:  "field\nvalue",
			obj:  (*struct{})(nil),
			err:  "strictcsv: destination (*struct {}) cannot be nil",
		},
		{
			name: "non-struct",
			csv:  "field\nvalue",
			obj:  0,
			err:  "strictcsv: destination (int) must be a non-nil pointer to a struct or slice of structs",
		},
		{
			name: "pointer to non-struct",
			csv:  "field\nvalue",
			obj:  new(int),
			err:  "strictcsv: destination (*int) must be a non-nil pointer to a struct or slice of structs",
		},
		{
			name: "no headers",
			csv:  "",
			obj:  &struct{}{},
			err:  "strictcsv: unable to read CSV headers: EOF",
		},
		{
			name: "unmapped header",
			csv:  "field",
			obj:  &struct{}{},
			err:  `strictcsv: CSV header "field" is not mapped to struct field`,
		},
		{
			name: "duplicate header",
			csv:  "field,field",
			obj:  &simple{},
			err:  `strictcsv: CSV header "field" is duplicated`,
		},
		{
			name: "unmapped field",
			csv:  "field",
			obj:  &unmapped{},
			err:  `strictcsv: field headers ["unmapped"] missing from CSV`,
		},
		{
			name: "ignores ignorable field",
			csv:  "field\nvalue\n",
			obj:  &ignored{},
			out:  &ignored{Field: "value"},
		},
		{
			name: "unable to read record",
			csv:  "field\n",
			obj:  &simple{},
			err:  "strictcsv: unable to read CSV record: EOF",
		},
		{
			name: "struct field rejected",
			csv:  "field\nvalue\n",
			obj:  &embedded{},
			err:  `strictcsv: field "Field" has unsupported type strictcsv.simple`,
		},
		{
			name: "missing field tag",
			csv:  "field\nvalue\n",
			obj:  &notag{},
			err:  `strictcsv: field "Field" missing csv tag`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := UnmarshalString(tt.csv, tt.obj)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.out, tt.obj)
		})
	}
}

func TestUnmarshalFailsToUnmarshalField(t *testing.T) {
	for _, s := range []any{
		&struct {
			Field int64 `csv:"field"`
		}{},
		&struct {
			Field uint64 `csv:"field"`
		}{},
		&struct {
			Field float64 `csv:"field"`
		}{},
		&struct {
			Field bool `csv:"field"`
		}{},
		&struct {
			Field badCSVField `csv:"field"`
		}{},
		&struct {
			Field badTextField `csv:"field"`
		}{},
	} {
		t.Logf("struct=%+v", s)
		err := UnmarshalString("field\nA", s)
		require.Error(t, err)
		require.Contains(t, err.Error(), `strictcsv: unable to unmarshal field "Field":`)
	}
}
