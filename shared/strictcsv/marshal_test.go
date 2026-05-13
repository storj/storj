// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	type simple struct {
		Field string `csv:"field"`
	}

	for _, tt := range []struct {
		name string
		in   any
		out  any
		err  string
	}{
		{
			name: "normal fields",
			in:   normalFields,
			out:  normalFieldsCSV,
		},
		{
			name: "optional fields unset",
			in:   optionalFieldsUnset,
			out:  optionalFieldsUnsetCSV,
		},
		{
			name: "optional fields set",
			in:   optionalFieldsSet,
			out:  optionalFieldsSetCSV,
		},
		{
			name: "defined fields",
			in:   definedFields,
			out:  definedFieldsCSV,
		},
		{
			name: "pointer to struct",
			in:   &simple{Field: "FIELD"},
			out:  "field\nFIELD\n",
		},
		{
			name: "slice of struct",
			in:   []simple{{Field: "FIELD1"}, {Field: "FIELD2"}},
			out:  "field\nFIELD1\nFIELD2\n",
		},
		{
			name: "slice of pointer to struct",
			in:   []*simple{{Field: "FIELD1"}, {Field: "FIELD2"}},
			out:  "field\nFIELD1\nFIELD2\n",
		},
		{
			name: "nil",
			in:   nil,
			err:  "strictcsv: source (<nil>) cannot be nil",
		},
		{
			name: "nil pointer to struct",
			in:   (*simple)(nil),
			err:  "strictcsv: source (*strictcsv.simple) cannot be nil",
		},
		{
			name: "not a struct, pointer to struct, or slice of structs",
			in:   0,
			err:  "strictcsv: source (int) must be a struct or slice of structs",
		},
		{
			name: "nil slice element",
			in:   []*simple{nil},
			err:  `strictcsv: slice has one or more nil elements`,
		},
		{
			name: "field cannot be struct",
			in: struct {
				Field simple `csv:"field"`
			}{},
			err: `strictcsv: field "Field" has unsupported type strictcsv.simple`,
		},
		{
			name: "field tag is missing",
			in:   struct{ Field int }{},
			err:  `strictcsv: field "Field" missing csv tag`,
		},
		{
			name: "ignores ignorable fields",
			in: struct {
				Field    string `csv:"field"`
				IgnoreMe string `csv:"-"`
			}{
				Field:    "value",
				IgnoreMe: "IGNOREME",
			},
			out: "field\nvalue\n",
		},
		{
			name: "unable to marshal field",
			in: struct {
				Field badCSVField `csv:"field"`
			}{},
			err: `strictcsv: unable to marshal field "Field": OHNO`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := MarshalString(tt.in)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}
