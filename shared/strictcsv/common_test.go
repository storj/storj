// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"bytes"
	"errors"
	"strings"
)

type NormalFields struct {
	String     string     `csv:"string"`
	Bool       bool       `csv:"bool"`
	Int64      int64      `csv:"int64"`
	Uint64     uint64     `csv:"uint64"`
	Float64    float64    `csv:"float64"`
	CustomCSV  CustomCSV  `csv:"custom-csv"`
	CustomText CustomText `csv:"custom-text"`
}

type OptionalFields struct {
	String     *string     `csv:"string"`
	Bool       *bool       `csv:"bool"`
	Int64      *int64      `csv:"int64"`
	Uint64     *uint64     `csv:"uint64"`
	Float64    *float64    `csv:"float64"`
	CustomCSV  *CustomCSV  `csv:"custom-csv"`
	CustomText *CustomText `csv:"custom-text"`
}

type DefinedFields struct {
	String  DefinedString  `csv:"string"`
	Bool    DefinedBool    `csv:"bool"`
	Int64   DefinedInt64   `csv:"int64"`
	Uint64  DefinedUint64  `csv:"uint64"`
	Float64 DefinedFloat64 `csv:"float64"`
}

type CustomCSV string

func (custom *CustomCSV) UnmarshalCSV(s string) error {
	*custom = CustomCSV(strings.TrimFunc(s, func(r rune) bool {
		return r == '~'
	}))
	return nil
}

func (custom CustomCSV) MarshalCSV() (string, error) {
	return "~" + string(custom) + "~", nil
}

type CustomText string

func (custom *CustomText) UnmarshalText(b []byte) error {
	*custom = CustomText(string(bytes.TrimFunc(b, func(r rune) bool {
		return r == '~'
	})))
	return nil
}

func (custom CustomText) MarshalText() ([]byte, error) {
	return []byte("~" + string(custom) + "~"), nil
}

type DefinedString string
type DefinedBool bool
type DefinedInt64 int64
type DefinedUint64 uint64
type DefinedFloat64 float64

var (
	normalFields = NormalFields{
		String:     "STRING",
		Bool:       true,
		Int64:      1,
		Uint64:     2,
		Float64:    3.3,
		CustomCSV:  "CUSTOMCSV",
		CustomText: "CUSTOMTEXT",
	}

	optionalFieldsUnset = OptionalFields{
		String:     nil,
		Bool:       nil,
		Int64:      nil,
		Uint64:     nil,
		Float64:    nil,
		CustomCSV:  nil,
		CustomText: nil,
	}

	optionalFieldsSet = OptionalFields{
		String:     optString("STRING"),
		Bool:       optBool(true),
		Int64:      optInt64(1),
		Uint64:     optUint64(2),
		Float64:    optFloat64(3.3),
		CustomCSV:  optCustomCSV("CUSTOMCSV"),
		CustomText: optCustomText("CUSTOMTEXT"),
	}

	definedFields = DefinedFields{
		String:  "STRING",
		Bool:    true,
		Int64:   1,
		Uint64:  2,
		Float64: 3.3,
	}

	normalFieldsCSV = `string,bool,int64,uint64,float64,custom-csv,custom-text
STRING,true,1,2,3.300000,~CUSTOMCSV~,~CUSTOMTEXT~
`

	optionalFieldsUnsetCSV = `string,bool,int64,uint64,float64,custom-csv,custom-text
,,,,,,
`
	optionalFieldsSetCSV = `string,bool,int64,uint64,float64,custom-csv,custom-text
STRING,true,1,2,3.300000,~CUSTOMCSV~,~CUSTOMTEXT~
`

	definedFieldsCSV = `string,bool,int64,uint64,float64
STRING,true,1,2,3.300000
`
)

func optString(v string) *string             { return &v }
func optInt64(v int64) *int64                { return &v }
func optUint64(v uint64) *uint64             { return &v }
func optFloat64(v float64) *float64          { return &v }
func optBool(v bool) *bool                   { return &v }
func optCustomCSV(v CustomCSV) *CustomCSV    { return &v }
func optCustomText(v CustomText) *CustomText { return &v }

type badCSVField struct{}

func (badCSVField) MarshalCSV() (string, error) {
	return "", errors.New("OHNO")
}

func (badCSVField) UnmarshalCSV(string) error {
	return errors.New("OHNO")
}

type badTextField struct{}

func (badTextField) MarshalText() ([]byte, error) {
	return nil, errors.New("OHNO")
}

func (badTextField) UnmarshalText([]byte) error {
	return errors.New("OHNO")
}
