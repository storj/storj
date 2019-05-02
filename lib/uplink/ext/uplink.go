// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go build -o uplink-cgo-common.so -buildmode=c-shared .

package main

// #cgo CFLAGS: -g -Wall
// #ifndef UPLINK_HEADERS
//   #define UPLINK_HEADERS
//   #include "uplink.h"
// #endif
import "C"
import (
	"context"
	"github.com/zeebo/errs"
	"reflect"

	"storj.io/storj/lib/uplink"
)

var ErrConvert = errs.Class("struct conversion error")
//func main() {}

//export NewUplink
func NewUplink(cConfig C.struct_Config, cErr *C.char) C.struct_Uplink {
	goConfig := new(uplink.Config)
	if err := GoToCStruct(cConfig, goConfig); err != nil {
		*cErr = *C.CString(err.Error())
	}
	//goConfig := uplink.Config{}
	//goConfig.Volatile.TLS.SkipPeerCAWhitelist = true
	//if err != nil {
	//
	//}

	goUplink, err := uplink.NewUplink(context.Background(), goConfig)
	//_, err := uplink.NewUplink(context.Background(), &goConfig)
	if err != nil {
		*cErr = *C.CString(err.Error())
	}

	//t := reflect.TypeOf(C.struct_Uplink{})
	//for i := 0; i < t.NumField(); i++ {
	//	//t := reflect.TypeOf(t.Field(i))
	//	field := t.Field(i)
	//	fmt.Printf("%+v\n", field)
	//	//fmt.Printf("field name: %s; kind: %s\n", field.Name, field.Type.Kind())
	//}
	return C.struct_Uplink{
		GoUplink: C.ulong(register(goUplink)),
		Config:   cConfig,
	}
	//return cConfig
	//fmt.Printf("go: %s\n", cUplink.volatile_.tls.SkipPeerCAWhitelist)
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

var cRegistry = make(map[uint64]interface{})
var cNext uint64 = 0

func register(value interface{}) uint64 {
	cNext += 1
	cRegistry[cNext] = value
	return cNext
}

func lookup(key uint64) interface{} {
	return cRegistry[key]
}
