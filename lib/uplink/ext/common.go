// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "uplink.h"
// #endif
import "C"
import (
	"reflect"

	"storj.io/storj/pkg/storj"
)

//var (
//	//export GetIDVersion
//	GetIDVersion = storj.GetIDVersion
//)

//export GetIDVersion
func GetIDVersion(number C.ushort, cErr *C.char) C.struct_IDVersion {
	cIDVersion := C.struct_IDVersion{}
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = *C.CString(err.Error())
		return C.struct_IDVersion{}
	}

	//cIDVersion := new(C.struct_IDVersion)
	if err := GoToCStruct(goIDVersion, cIDVersion); err != nil {
		*cErr = *C.CString(err.Error())
		return cIDVersion
	}
	return cIDVersion
}

func GoToCStruct(fromVar, toPtr interface{}) error {
	fromValue := reflect.ValueOf(fromVar)
	fromKind := fromValue.Kind()
	toValue := reflect.Indirect(reflect.ValueOf(toPtr))

	switch fromKind {
	case reflect.String:
		toValue.Set(reflect.ValueOf(C.CString(fromValue.String())))
		return nil
	case reflect.Bool:
		toValue.Set(reflect.ValueOf(C.bool(fromValue.Bool())))
		return nil
	case reflect.Int:
		toValue.Set(reflect.ValueOf(C.int(fromValue.Int())))
		return nil
	case reflect.Uint:
		toValue.Set(reflect.ValueOf(C.uint(fromValue.Uint())))
		return nil
	//case reflect.Uintptr:
	//	toValue.Set(reflect.ValueOf(C.GoUintptr(fromValue.Uint())))
	//	return nil
	case reflect.Struct:
		for i := 0; i < fromValue.NumField(); i++ {
			fromFieldValue := fromValue.Field(i)
			fromField := fromValue.Type().Field(i)
			toField := toValue.FieldByName(fromField.Name)
			toFieldPtr := reflect.New(toField.Type())
			toFieldValue := toFieldPtr.Interface()

			// initialize new C value pointer
			if err := GoToCStruct(fromFieldValue.Interface(), toFieldValue); err != nil {
				return err
			}

			// set "to" field value to modified value
			toValue.FieldByName(fromField.Name).Set(reflect.Indirect(toFieldPtr))
		}
		return nil
	default:
		return ErrConvert.New("unsupported kind %s", fromKind)
	}
}

func CToGoStruct(fromVar, toPtr interface{}) error {
	fromValue := reflect.ValueOf(fromVar)
	fromType := fromValue.Type()

	toValue := reflect.Indirect(reflect.ValueOf(toPtr))

	//fmt.Printf("fromValue: %+v\n", fromValue)
	//fmt.Printf("toValue: %+v\n", toValue)

	switch {
	case fromType == reflect.TypeOf(C.CString("")):
		toValue.Set(reflect.ValueOf(C.GoString(fromValue.Interface().(*C.char))))
		return nil
	case fromType == reflect.TypeOf(C.bool(false)):
		toValue.Set(reflect.ValueOf(bool(fromValue.Interface().(C.bool))))
		return nil
	case fromType == reflect.TypeOf(C.int(0)):
		toValue.Set(reflect.ValueOf(int(fromValue.Interface().(C.int))))
		return nil
	case fromType == reflect.TypeOf(C.uint(0)):
		toValue.Set(reflect.ValueOf(uint(fromValue.Interface().(C.uint))))
		return nil
	//case reflect.Uintptr:
	//	toValue.Set()
	//	return nil
	case fromType.Kind() == reflect.Struct:
		for i := 0; i < fromValue.NumField(); i++ {
			fromFieldValue := fromValue.Field(i)
			fromField := fromValue.Type().Field(i)
			toField := toValue.FieldByName(fromField.Name)

			//fmt.Printf("toField: %+v\n", toField)

			toFieldPtr := reflect.New(toField.Type())
			toFieldValue := toFieldPtr.Interface()

			// initialize new Go value pointer
			if err := CToGoStruct(fromFieldValue.Interface(), toFieldValue); err != nil {
				return err
			}

			// set "to" field value to modified value
			toValue.FieldByName(fromField.Name).Set(reflect.Indirect(toFieldPtr))
		}
		return nil
	default:
		return ErrConvert.New("unsupported type %s", fromType)
	}
}

