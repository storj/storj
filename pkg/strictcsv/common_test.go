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
	Int        int        `csv:"int"`
	Int8       int8       `csv:"int8"`
	Int16      int16      `csv:"int16"`
	Int32      int32      `csv:"int32"`
	Int64      int64      `csv:"int64"`
	Uint       uint       `csv:"uint"`
	Uint8      uint8      `csv:"uint8"`
	Uint16     uint16     `csv:"uint16"`
	Uint32     uint32     `csv:"uint32"`
	Uint64     uint64     `csv:"uint64"`
	Float32    float32    `csv:"float32"`
	Float64    float64    `csv:"float64"`
	Bool       bool       `csv:"bool"`
	CustomCSV  CustomCSV  `csv:"custom-csv"`
	CustomText CustomText `csv:"custom-text"`
}

type OptionalFields struct {
	String     *string     `csv:"string"`
	Int        *int        `csv:"int"`
	Int8       *int8       `csv:"int8"`
	Int16      *int16      `csv:"int16"`
	Int32      *int32      `csv:"int32"`
	Int64      *int64      `csv:"int64"`
	Uint       *uint       `csv:"uint"`
	Uint8      *uint8      `csv:"uint8"`
	Uint16     *uint16     `csv:"uint16"`
	Uint32     *uint32     `csv:"uint32"`
	Uint64     *uint64     `csv:"uint64"`
	Float32    *float32    `csv:"float32"`
	Float64    *float64    `csv:"float64"`
	Bool       *bool       `csv:"bool"`
	CustomCSV  *CustomCSV  `csv:"custom-csv"`
	CustomText *CustomText `csv:"custom-text"`
}

type DefinedFields struct {
	String  DefinedString  `csv:"string"`
	Int     DefinedInt     `csv:"int"`
	Int8    DefinedInt8    `csv:"int8"`
	Int16   DefinedInt16   `csv:"int16"`
	Int32   DefinedInt32   `csv:"int32"`
	Int64   DefinedInt64   `csv:"int64"`
	Uint    DefinedUint    `csv:"uint"`
	Uint8   DefinedUint8   `csv:"uint8"`
	Uint16  DefinedUint16  `csv:"uint16"`
	Uint32  DefinedUint32  `csv:"uint32"`
	Uint64  DefinedUint64  `csv:"uint64"`
	Float32 DefinedFloat32 `csv:"float32"`
	Float64 DefinedFloat64 `csv:"float64"`
	Bool    DefinedBool    `csv:"bool"`
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
type DefinedInt int
type DefinedInt8 int8
type DefinedInt16 int16
type DefinedInt32 int32
type DefinedInt64 int64
type DefinedUint uint
type DefinedUint8 uint8
type DefinedUint16 uint16
type DefinedUint32 uint32
type DefinedUint64 uint64
type DefinedFloat32 float32
type DefinedFloat64 float64
type DefinedBool bool

var (
	normalFields = NormalFields{
		String:     "STRING",
		Int:        1,
		Int8:       2,
		Int16:      3,
		Int32:      4,
		Int64:      5,
		Uint:       6,
		Uint8:      7,
		Uint16:     8,
		Uint32:     9,
		Uint64:     10,
		Float32:    11.11,
		Float64:    12.12,
		Bool:       true,
		CustomCSV:  "CUSTOMCSV",
		CustomText: "CUSTOMTEXT",
	}

	optionalFieldsUnset = OptionalFields{
		String:     nil,
		Int:        nil,
		Int8:       nil,
		Int16:      nil,
		Int32:      nil,
		Int64:      nil,
		Uint:       nil,
		Uint8:      nil,
		Uint16:     nil,
		Uint32:     nil,
		Uint64:     nil,
		Float32:    nil,
		Float64:    nil,
		Bool:       nil,
		CustomCSV:  nil,
		CustomText: nil,
	}

	optionalFieldsSet = OptionalFields{
		String:     optString("STRING"),
		Int:        optInt(1),
		Int8:       optInt8(2),
		Int16:      optInt16(3),
		Int32:      optInt32(4),
		Int64:      optInt64(5),
		Uint:       optUint(6),
		Uint8:      optUint8(7),
		Uint16:     optUint16(8),
		Uint32:     optUint32(9),
		Uint64:     optUint64(10),
		Float32:    optFloat32(11.11),
		Float64:    optFloat64(12.12),
		Bool:       optBool(true),
		CustomCSV:  optCustomCSV("CUSTOMCSV"),
		CustomText: optCustomText("CUSTOMTEXT"),
	}

	definedFields = DefinedFields{
		String:  "STRING",
		Int:     1,
		Int8:    2,
		Int16:   3,
		Int32:   4,
		Int64:   5,
		Uint:    6,
		Uint8:   7,
		Uint16:  8,
		Uint32:  9,
		Uint64:  10,
		Float32: 11.11,
		Float64: 12.12,
		Bool:    true,
	}

	normalFieldsCSV = `string,int,int8,int16,int32,int64,uint,uint8,uint16,uint32,uint64,float32,float64,bool,custom-csv,custom-text
STRING,1,2,3,4,5,6,7,8,9,10,11.110000,12.120000,true,~CUSTOMCSV~,~CUSTOMTEXT~
`

	optionalFieldsUnsetCSV = `string,int,int8,int16,int32,int64,uint,uint8,uint16,uint32,uint64,float32,float64,bool,custom-csv,custom-text
,,,,,,,,,,,,,,,
`
	optionalFieldsSetCSV = `string,int,int8,int16,int32,int64,uint,uint8,uint16,uint32,uint64,float32,float64,bool,custom-csv,custom-text
STRING,1,2,3,4,5,6,7,8,9,10,11.110000,12.120000,true,~CUSTOMCSV~,~CUSTOMTEXT~
`

	definedFieldsCSV = `string,int,int8,int16,int32,int64,uint,uint8,uint16,uint32,uint64,float32,float64,bool
STRING,1,2,3,4,5,6,7,8,9,10,11.110000,12.120000,true
`
)

func optString(v string) *string { return &v }

func optInt(v int) *int { return &v }

func optInt8(v int8) *int8 { return &v }

func optInt16(v int16) *int16 { return &v }

func optInt32(v int32) *int32 { return &v }

func optInt64(v int64) *int64 { return &v }

func optUint(v uint) *uint { return &v }

func optUint8(v uint8) *uint8 { return &v }

func optUint16(v uint16) *uint16 { return &v }

func optUint32(v uint32) *uint32 { return &v }

func optUint64(v uint64) *uint64 { return &v }

func optFloat32(v float32) *float32 { return &v }

func optFloat64(v float64) *float64 { return &v }

func optBool(v bool) *bool { return &v }

func optCustomCSV(v CustomCSV) *CustomCSV { return &v }

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
