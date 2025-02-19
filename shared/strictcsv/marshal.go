// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"bytes"
	"encoding"
	"encoding/csv"
	"io"
	"reflect"
	"strconv"
)

var (
	marshalCSVType  = reflect.TypeOf((*Marshaler)(nil)).Elem()
	marshalTextType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

	stringType  = reflect.TypeOf((*string)(nil)).Elem()
	boolType    = reflect.TypeOf((*bool)(nil)).Elem()
	int64Type   = reflect.TypeOf((*int64)(nil)).Elem()
	uint64Type  = reflect.TypeOf((*uint64)(nil)).Elem()
	float64Type = reflect.TypeOf((*float64)(nil)).Elem()
)

// Marshaler is used to implement customized CSV field marshaling.
type Marshaler interface {
	MarshalCSV() (string, error)
}

// Marshal marshals an object into CSV and returns the bytes.
func Marshal(obj interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := Write(buf, obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalString marshals an object into CSV and returns a string.
func MarshalString(obj interface{}) (string, error) {
	buf := new(bytes.Buffer)
	if err := Write(buf, obj); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Write marshals an object into CSV and writes it to the writer.
func Write(w io.Writer, obj interface{}) error {
	if obj == nil {
		return Error.New("source (%T) cannot be nil", obj)
	}
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return Error.New("source (%T) cannot be nil", obj)
	}

	t := v.Type()
	isSlice := false
	if t.Kind() == reflect.Slice {
		isSlice = true
		t = t.Elem()
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Error.New("source (%T) must be a struct or slice of structs", obj)
	}

	fields, err := getGettableFields(t)
	if err != nil {
		return err
	}

	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		headers = append(headers, field.Header)
	}

	csvw := csv.NewWriter(w)
	if err := csvw.Write(headers); err != nil {
		return Error.Wrap(err)
	}

	if !isSlice {
		record, err := getFieldsRecord(fields, v)
		if err != nil {
			return err
		}
		if err := csvw.Write(record); err != nil {
			return Error.Wrap(err)
		}
	} else {
		for i := 0; i < v.Len(); i++ {
			record, err := getFieldsRecord(fields, v.Index(i))
			if err != nil {
				return err
			}
			if err := csvw.Write(record); err != nil {
				return Error.Wrap(err)
			}
		}
	}

	csvw.Flush()
	if err := csvw.Error(); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func getFieldsRecord(fields []gettableField, v reflect.Value) ([]string, error) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, Error.New("slice has one or more nil elements")
		}
		v = v.Elem()
	}
	record := make([]string, 0, len(fields))
	for _, field := range fields {
		column, err := field.Getter(v.FieldByIndex(field.Index))
		if err != nil {
			return nil, Error.New("unable to marshal field %q: %v", field.Name, err)
		}
		record = append(record, column)
	}
	return record, nil

}

type gettableField struct {
	Header string
	Name   string
	Index  []int
	Getter func(v reflect.Value) (string, error)
}

func getGettableFields(t reflect.Type) ([]gettableField, error) {
	var fields []gettableField
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		header := field.Tag.Get("csv")
		if header == "" {
			return nil, Error.New("field %q missing csv tag", field.Name)
		}
		if header == "-" {
			continue
		}

		var getter func(reflect.Value) (string, error)
		switch {
		case field.Type.Implements(marshalCSVType):
			getter = getMarshalCSVValue
		case reflect.PointerTo(field.Type).Implements(marshalCSVType):
			getter = getMarshalCSVValue
		case field.Type.Implements(marshalTextType):
			getter = getMarshalTextValue
		case reflect.PointerTo(field.Type).Implements(marshalTextType):
			getter = getMarshalTextValue
		default:
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			switch ft.Kind() {
			case reflect.String:
				getter = getStringValue
			case reflect.Bool:
				getter = getBoolValue
			case reflect.Int64:
				getter = getInt64Value
			case reflect.Uint64:
				getter = getUint64Value
			case reflect.Float64:
				getter = getFloat64Value
			default:
				return nil, Error.New("field %q has unsupported type %s", field.Name, field.Type.String())
			}
		}

		if field.Type.Kind() == reflect.Ptr {
			getter = getPointerValue(getter)
		}

		fields = append(fields, gettableField{
			Header: header,
			Name:   field.Name,
			Index:  field.Index,
			Getter: getter,
		})
	}
	return fields, nil
}

func getPointerValue(getter func(reflect.Value) (string, error)) func(reflect.Value) (string, error) {
	return func(v reflect.Value) (string, error) {
		if v.IsNil() {
			return "", nil
		}
		return getter(v.Elem())
	}
}

func getStringValue(v reflect.Value) (string, error) {
	return v.Convert(stringType).Interface().(string), nil
}

func getInt64Value(v reflect.Value) (string, error) {
	return strconv.FormatInt(v.Convert(int64Type).Interface().(int64), 10), nil
}

func getUint64Value(v reflect.Value) (string, error) {
	return strconv.FormatUint(v.Convert(uint64Type).Interface().(uint64), 10), nil
}

func getFloat64Value(v reflect.Value) (string, error) {
	return strconv.FormatFloat(v.Convert(float64Type).Interface().(float64), 'f', 6, 64), nil
}

func getBoolValue(v reflect.Value) (string, error) {
	return strconv.FormatBool(v.Convert(boolType).Interface().(bool)), nil
}

func getMarshalCSVValue(v reflect.Value) (string, error) {
	value := v.Interface().(Marshaler)
	return value.MarshalCSV()
}

func getMarshalTextValue(v reflect.Value) (string, error) {
	value := v.Interface().(encoding.TextMarshaler)
	b, err := value.MarshalText()
	return string(b), err
}
