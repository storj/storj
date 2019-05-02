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
	"fmt"
	"reflect"

	"storj.io/storj/lib/uplink"
)

//func main() {}

//export NewUplink
func NewUplink(cConfig C.struct_Config, cErr *C.char) C.struct_Uplink {
	goConfig := new(uplink.Config)
	if err := ConvertStruct(cConfig, goConfig); err != nil {
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

func ConvertStruct(fromVar, toPtr interface{}) error {
	fromValue := reflect.ValueOf(fromVar)
	fromKind := fromValue.Kind()
	//fmt.Printf("from kind: %s\n", fromValue.Kind())
	toValue := reflect.Indirect(reflect.ValueOf(toPtr))
	//fmt.Printf("toValue kind: %s\n", toValue.Kind())

	switch fromKind {
	case reflect.String:
		toValue.Set(reflect.ValueOf(C.CString(fromValue.String())))
		return nil
	case reflect.Bool:
		toValue.Set(reflect.ValueOf(C.bool(fromValue.Bool())))
		return nil
	case reflect.Uintptr:
	case reflect.Struct:
	default:
		panic(fmt.Sprintf("unsupported kind %s", fromKind))
	}

	for i := 0; i < fromValue.NumField(); i++ {
		fromFieldValue := fromValue.Field(i)

		// TODO: recurse here?
		switch fromFieldValue.Kind() {
		case reflect.Uintptr:
		case reflect.Struct:
			fmt.Printf("fromField: %+v\n", fromFieldValue)

			toFieldPtr := reflect.New(fromFieldValue.Type())
			if err := ConvertStruct(fromFieldValue.Interface(), toFieldPtr.Pointer()); err != nil {
				return err
			}
			//toValue.
			// -- use .Set
			//fromField := fromType.Field(i)
			//fmt.Printf("fromField: %+v\n", fromField)
			//toValue.FieldByName(fromField.Name)
			//fromField := fromValue.Field(i)
		//fmt.Printf("fromField: %+v\n", fromFieldValue)
			//toValue.FieldByName(fromField.Name)
		default:
			//fmt.Println("default case!")
		}
	}
	return nil
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
