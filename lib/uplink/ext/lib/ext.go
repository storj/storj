// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package lib

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "../uplink.h"
// #endif
import "C"
import (
	"fmt"
	"reflect"

	"github.com/zeebo/errs"
)

var ErrConvert = errs.Class("struct conversion error")

func GoToCStruct(fromVar, toPtr interface{}) error {
	fromValue := reflect.ValueOf(fromVar)
	fromKind := fromValue.Kind()
	toPtrValue := reflect.ValueOf(toPtr)

	conversionFunc := fromValue.MethodByName("GoToC")
	if conversionFunc.IsValid() {
		return conversionFunc.Call([]reflect.Value{toPtrValue})[0].Interface().(error)
	}

	toValue := reflect.Indirect(toPtrValue)

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
	case reflect.Uint8:
		//fmt.Printf("fromValue: \n", reflect.ValueOf(C.uint(fromValue.Uint())))
		//fmt.Printf("fromValue type: \n", reflect.TypeOf(C.uint(fromValue.Uint())))
		//fmt.Printf("toValue: \n", reflect.ValueOf(toValue))
		//fmt.Printf("toValue type: %+v\n", toValue.Type().Name())
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
			if toField.CanSet() {
				//fmt.Printf("toField type: %+v\n", toField.Type().Name())
				toFieldPtr := reflect.New(toField.Type())
				toFieldValue := toFieldPtr.Interface()

				// initialize new C value pointer
				if err := GoToCStruct(fromFieldValue.Interface(), toFieldValue); err != nil {
					return err
				}

				// set "to" field value to modified value
				toValue.FieldByName(fromField.Name).Set(reflect.Indirect(toFieldPtr))
			}
		}
		return nil
	default:
		return ErrConvert.New("unsupported kind %s", fromKind)
	}
}

func CToGoStruct(fromVar, toPtr interface{}) error {
	fromValue := reflect.ValueOf(fromVar)
	fromType := fromValue.Type()
	toPtrValue := reflect.ValueOf(toPtr)
	toValue := reflect.Indirect(toPtrValue)

	fmt.Printf("fromValue type: %s\n", fromValue.Type().Name())
	fmt.Printf("toValue type: %s\n", toValue.Type().Name())
	conversionFunc := toPtrValue.MethodByName("CToGo")
	fmt.Printf("convoersioniFunc is valid?: %v\n", conversionFunc.IsValid())
	if conversionFunc.IsValid() {
		result := conversionFunc.Call([]reflect.Value{fromValue})[0].Interface()
		if err, ok := result.(error); ok && err != nil {
			return err
		}
		return nil
	}

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
	case fromType == reflect.TypeOf(C.uchar('a')):
		//fmt.Printf("toValue type: %s\n", toValue.Type().Name())
		//fmt.Printf("toValue char: %d\n", toValue.Uint())
		toValue.Set(reflect.ValueOf(uint8(fromValue.Uint())))
		return nil
	//case fromType == reflect.TypeOf(C.ulong(0)):
	//	toValue.Set(reflect.ValueOf(fromValue.Uint()))
	//	return nil
	//case fromType == reflect.TypeOf(C.long(0)):
	//	toValue.Set(reflect.ValueOf(int64(fromValue.Int())))
	//	return nil
	//case reflect.Uintptr:
	//	toValue.Set()
	//	return nil
	//case fromType == reflect.TypeOf(C.uchar('a')):
	case fromType.Kind() == reflect.Struct:
		for i := 0; i < fromValue.NumField(); i++ {
			fromFieldValue := fromValue.Field(i)
			fromField := fromValue.Type().Field(i)
			toField := toValue.FieldByName(fromField.Name)

			if toField.CanSet() {
				//fmt.Printf("toField: %+v\n", toField)

				//fmt.Printf("toField type: %+v\n", toField.Type().Name())
				toFieldPtr := reflect.New(toField.Type())
				toFieldValue := toFieldPtr.Interface()

				// initialize new Go value pointer
				if err := CToGoStruct(fromFieldValue.Interface(), toFieldValue); err != nil {
					return err
				}

				// set "to" field value to modified value
				toValue.FieldByName(fromField.Name).Set(reflect.Indirect(toFieldPtr))
			}
		}
		return nil
	default:
		return ErrConvert.New("unsupported type %s", fromType)
	}
}
