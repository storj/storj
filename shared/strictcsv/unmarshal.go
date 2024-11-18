// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"bytes"
	"encoding"
	"encoding/csv"
	"errors"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var (
	unmarshalCSVType  = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	unmarshalTextType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// Unmarshaler is used to implement customized CSV field unmarshaling.
type Unmarshaler interface {
	UnmarshalCSV(s string) error
}

// Unmarshal unmarshals an object from CSV bytes.
func Unmarshal(b []byte, obj interface{}) error {
	return Read(bytes.NewReader(b), obj)
}

// UnmarshalString unmarshals an object from a CSV string.
func UnmarshalString(s string, obj interface{}) error {
	return Read(strings.NewReader(s), obj)
}

// Read unmarshals an object from a CSV reader.
func Read(r io.Reader, obj interface{}) error {
	pv := reflect.ValueOf(obj)
	switch {
	case pv == reflect.Value{}:
		return Error.New("destination (%T) cannot be nil", obj)
	case pv.Kind() != reflect.Ptr:
		return Error.New("destination (%T) must be a non-nil pointer to a struct or slice of structs", obj)
	case pv.IsNil():
		return Error.New("destination (%T) cannot be nil", obj)
	}

	isSlice := false
	isPtr := false

	v := pv.Elem()
	t := v.Type()
	if t.Kind() == reflect.Slice {
		isSlice = true
		t = t.Elem()
	}
	if t.Kind() == reflect.Ptr {
		isPtr = true
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Error.New("destination (%T) must be a non-nil pointer to a struct or slice of structs", obj)
	}

	settableFields, err := getSettableFields(t)
	if err != nil {
		return err
	}

	csvr := csv.NewReader(r)

	headers, err := csvr.Read()
	if err != nil {
		return Error.New("unable to read CSV headers: %v", err)
	}

	unmatchedFields := make(map[string]struct{}, len(settableFields))
	for header := range settableFields {
		unmatchedFields[header] = struct{}{}
	}

	fields := make([]settableField, 0, len(headers))
	for _, header := range headers {
		field, ok := settableFields[header]
		if !ok {
			return Error.New("CSV header %q is not mapped to struct field", header)
		}
		if _, ok := unmatchedFields[header]; !ok {
			return Error.New("CSV header %q is duplicated", header)
		}
		delete(unmatchedFields, header)
		fields = append(fields, field)
	}
	if len(unmatchedFields) > 0 {
		unmatchedHeaders := make([]string, 0, len(unmatchedFields))
		for header := range unmatchedFields {
			unmatchedHeaders = append(unmatchedHeaders, header)
		}
		sort.Strings(unmatchedHeaders)
		return Error.New("field headers %q missing from CSV", unmatchedHeaders)
	}

	if !isSlice {
		record, err := csvr.Read()
		if err != nil {
			return Error.New("unable to read CSV record: %v", err)
		}
		if isPtr {
			v = reflect.New(t).Elem()
			pv.Elem().Set(v.Addr())
		}
		return setFields(fields, record, v)
	}

	for {
		record, err := csvr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				pv.Elem().Set(v)
				return nil
			}
			return Error.New("unable to read CSV record: %v", err)
		}

		e := reflect.New(t).Elem()
		// This length check is purely defensive since the CSV reader already
		// handles this.
		if len(record) != len(fields) {
			return Error.New("expected %d columns; got %d", len(fields), len(record))
		}
		if err := setFields(fields, record, e); err != nil {
			return err
		}
		if isPtr {
			v = reflect.Append(v, e.Addr())
		} else {
			v = reflect.Append(v, e)
		}
	}
}

type settableField struct {
	Name   string
	Index  []int
	Setter func(v reflect.Value, s string) error
}

type settableFields map[string]settableField

func getSettableFields(t reflect.Type) (settableFields, error) {
	fields := make(settableFields)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		header := field.Tag.Get("csv")
		if header == "" {
			return nil, Error.New("field %q missing csv tag", field.Name)
		}
		if header == "-" {
			continue
		}

		var setter func(reflect.Value, string) error
		switch {
		case field.Type.Implements(unmarshalCSVType):
			setter = setUnmarshalCSVValue
		case reflect.PointerTo(field.Type).Implements(unmarshalCSVType):
			setter = setUnmarshalCSVValue
		case field.Type.Implements(unmarshalTextType):
			setter = setUnmarshalTextValue
		case reflect.PointerTo(field.Type).Implements(unmarshalTextType):
			setter = setUnmarshalTextValue
		default:
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			switch ft.Kind() {
			case reflect.String:
				setter = setStringValue
			case reflect.Bool:
				setter = setBoolValue
			case reflect.Int64:
				setter = setInt64Value
			case reflect.Uint64:
				setter = setUint64Value
			case reflect.Float64:
				setter = setFloat64Value
			default:
				return nil, Error.New("field %q has unsupported type %s", field.Name, field.Type.String())
			}
		}
		if field.Type.Kind() == reflect.Ptr {
			setter = setPointerValue(setter)
		}
		fields[header] = settableField{
			Name:   field.Name,
			Index:  field.Index,
			Setter: setter,
		}
	}
	return fields, nil
}

func setFields(fields []settableField, record []string, v reflect.Value) error {
	for i, field := range fields {
		if err := field.Setter(v.FieldByIndex(field.Index), record[i]); err != nil {
			return Error.New("unable to unmarshal field %q: %v", field.Name, err)
		}
	}
	return nil
}

func setPointerValue(setter func(reflect.Value, string) error) func(reflect.Value, string) error {
	return func(v reflect.Value, s string) error {
		if s == "" {
			return nil
		}
		n := reflect.New(v.Type().Elem())
		if err := setter(n.Elem(), s); err != nil {
			return err
		}
		v.Set(n)
		return nil
	}
}

func setStringValue(v reflect.Value, s string) error {
	v.Set(reflect.ValueOf(s).Convert(v.Type()))
	return nil
}

func setBoolValue(v reflect.Value, s string) error {
	value, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(value).Convert(v.Type()))
	return nil
}

func setInt64Value(v reflect.Value, s string) error {
	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(value).Convert(v.Type()))
	return nil
}

func setUint64Value(v reflect.Value, s string) error {
	value, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(value).Convert(v.Type()))
	return nil
}

func setFloat64Value(v reflect.Value, s string) error {
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(value).Convert(v.Type()))
	return nil
}

func setUnmarshalCSVValue(v reflect.Value, s string) error {
	value := v.Addr().Interface().(Unmarshaler)
	return value.UnmarshalCSV(s)
}

func setUnmarshalTextValue(v reflect.Value, s string) error {
	value := v.Addr().Interface().(encoding.TextUnmarshaler)
	return value.UnmarshalText([]byte(s))
}
