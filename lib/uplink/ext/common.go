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
	"context"
	"reflect"
	"unsafe"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/storj"
)

var (
	// cgo types
	cStringType = reflect.TypeOf(C.CString(""))
	cBoolType   = reflect.TypeOf(C.bool(false))
	cIntType    = reflect.TypeOf(C.int(0))
	cUintType   = reflect.TypeOf(C.uint(0))
	// NB: C.uchar is uint8
	cUcharType  = reflect.TypeOf(C.uchar('0'))
	// NB: C.long is int64
	cLongType   = reflect.TypeOf(C.long(0))

	// our types
	memorySizeType = reflect.TypeOf(memory.Size(0))
	keyPtrType     = reflect.TypeOf(new(C.Key))

	ErrConvert = errs.Class("struct conversion error")
	ctxMapping = newMapping()
	IDVersionMapping = newMapping()
)

// Create pointer to a go struct for C to interact with
func cPointerFromGoStruct(s interface{}) C.GoUintptr {
	return C.GoUintptr(reflect.ValueOf(s).Pointer())
}

func goPointerFromCGoUintptr(p C.GoUintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p))
}

// GoCtxPtr gets a pointer to a go context that can be passed around
// TODO: Get rid of this and just use context.Background() for everything in go land
//export GetContext
func GetContext() C.GoCtxPtr {
	ctx := context.Background()
	return C.GoCtxPtr(ctxMapping.Add(ctx))
}

//export GetIDVersion
func GetIDVersion(number C.uint, cErr **C.char) (cIDVersion C.IDVersion) {
	goIDVersion, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cErr = C.CString(err.Error())
		return cIDVersion
	}

	return C.IDVersion(IDVersionMapping.Add(goIDVersion))
}

//export GetIDVersionNumber
func GetIDVersionNumber(idversion C.IDVersion) (IDVersionNumber C.IDVersionNumber) {
	goApiKeyStruct, ok := IDVersionMapping.Get(token(idversion)).(storj.IDVersion)
	if !ok {
		return IDVersionNumber
	}

	return C.IDVersionNumber(goApiKeyStruct.Number)
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
	case keyPtrType:
		key := new(storj.Key)
		from := C.GoBytes(unsafe.Pointer(fromValue.Interface().(*C.Key)), 32)
		copy(key[:], from)
		toValue.Set(reflect.ValueOf(key))
		return nil
	case cBoolType:
		toValue.Set(reflect.ValueOf(fromValue.Bool()))
		return nil
	case cIntType:
		toValue.Set(reflect.ValueOf(int(fromValue.Int())))
		return nil
	case cUintType:
		toValue.Set(reflect.ValueOf(uint(fromValue.Uint())))
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
