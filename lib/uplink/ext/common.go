// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "headers/main.h"
// #endif
import "C"
import (
	"reflect"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/storj"
)

var (
	cStringType    = reflect.TypeOf(C.CString(""))
	cBoolType      = reflect.TypeOf(C.bool(false))
	cIntType       = reflect.TypeOf(C.int(0))
	cUintType      = reflect.TypeOf(C.uint(0))
	cUcharType     = reflect.TypeOf(C.uchar('0'))
	cLongType      = reflect.TypeOf(C.long(0))
	memorySizeType = reflect.TypeOf(memory.Size(0))

	ErrConvert = errs.Class("struct conversion error")
)

//export GetIDVersion
func GetIDVersion(number C.uint, cErr *C.char) C.struct_IDVersion {
	cIDVersion := C.struct_IDVersion{}
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = *C.CString(err.Error())
		return cIDVersion
	}

	return C.struct_IDVersion{
		GoIDVersion: C.GoUintptr(reflect.ValueOf(&goIDVersion).Pointer()),
		// NB: C.uchar is uint8
		Number:      C.uchar(goIDVersion.Number),
	}
}

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
		toValue.Set(reflect.ValueOf(C.uint(fromValue.Uint())))
		return nil
	case reflect.Struct:
		for i := 0; i < fromValue.NumField(); i++ {
			fromFieldValue := fromValue.Field(i)
			fromField := fromValue.Type().Field(i)
			toField := toValue.FieldByName(fromField.Name)
			if toField.CanSet() {
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

	conversionFunc := toPtrValue.MethodByName("CToGo")
	if conversionFunc.IsValid() {
		result := conversionFunc.Call([]reflect.Value{fromValue})[0].Interface()
		if err, ok := result.(error); ok && err != nil {
			return err
		}
		return nil
	}

	switch fromType {
	case cStringType:
		toValue.Set(reflect.ValueOf(C.GoString(fromValue.Interface().(*C.char))))
		return nil
	case cBoolType:
		toValue.Set(reflect.ValueOf(bool(fromValue.Interface().(C.bool))))
		return nil
	case cIntType:
		toValue.Set(reflect.ValueOf(int(fromValue.Interface().(C.int))))
		return nil
	case cUintType:
		// TODO: simplify? ^ as well
		toValue.Set(reflect.ValueOf(uint(fromValue.Interface().(C.uint))))
		return nil
	case cUcharType:
		toValue.Set(reflect.ValueOf(uint8(fromValue.Uint())))
		return nil
	case cLongType:
		switch toValue.Type() {
		case memorySizeType:
			// TODO: can casting be done with reflection?
			toValue.Set(reflect.ValueOf(memory.Size(fromValue.Int())))
		default:
			toValue.Set(reflect.ValueOf(int64(fromValue.Int())))
		}
		return nil
	default:
		if fromType.Kind() == reflect.Struct {
			for i := 0; i < fromValue.NumField(); i++ {
				fromFieldValue := fromValue.Field(i)
				fromField := fromValue.Type().Field(i)
				toField := toValue.FieldByName(fromField.Name)

				if toField.CanSet() {
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
		}

		return ErrConvert.New("unsupported type %s", fromType)
	}
}
