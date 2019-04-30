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

func main() {}

//export NewUplink
func NewUplink(cConfig C.struct_Config, cErr *C.char) C.struct_Uplink {
	goConfig := new(uplink.Config)
	if err := cToGoStruct(cConfig, goConfig); err != nil {
		*cErr = *C.CString(err.Error())
	}
	//goConfig := uplink.Config{}
	goConfig.Volatile.TLS.SkipPeerCAWhitelist = true
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

func cToGoStruct(cStruct interface{}, goPtr interface{}) error {
	cStructValue := reflect.ValueOf(cStruct)
	switch cStructValue.Kind() {
	case reflect.Uintptr:
		fmt.Println("Uintptr case!")
	case reflect.Struct:
		fmt.Println("outer struct case!")
		goFieldI := cStructValue.Interface()
		//elem := goFieldValue.Elem()
		fmt.Printf("%+v\n", goFieldI)
		//fmt.Printf("%+v\n", elem)
		//if err := cToGoStruct(goFieldI, reflect.New(cStructValue.Type())); err != nil {
		//	return err
		//}
	default:
		fmt.Println("outer default case!")
		//reflect.Indirect(reflect.ValueOf(goPtr))
		reflect.Indirect(reflect.ValueOf(goPtr))
		v := reflect.ValueOf(goPtr)
		v.Pointer()
	}

	//for i := 0; i < cStructValue.NumField(); i++ {
	//	field := cStructValue.Field(i)
	//	fmt.Printf("%+v\n", field)
	//
	//	//fmt.Printf("%s\n", field.Name)
	//	goPtrValue := reflect.ValueOf(goPtr)
	//	//goValue := reflect.Indirect(goPtrValue)
	//	goValue := reflect.Indirect(goPtrValue)
	//	goFieldValue := goValue.FieldByName(field.Name)
	//
	//	fmt.Printf("kind: %+v\n", goFieldValue.Kind())
	//	//fmt.Printf("type: %+v\n", goValue.Type())
	//	fmt.Printf("type: %+v\n", goValue.Type())
	//	//fmt.Printf("type: %+v\n", goPtrValue.Type().Name())
	//	switch goFieldValue.Kind() {
	//	case reflect.Uintptr:
	//		fmt.Println("Uintptr case!")
	//		//goFieldValue.
	//	case reflect.Struct:
	//		fmt.Println("struct case!")
	//		goFieldI := goFieldValue.Interface()
	//		//elem := goFieldValue.Elem()
	//		fmt.Printf("%+v\n", goFieldI)
	//		//fmt.Printf("%+v\n", elem)
	//		//if err := cToGoStruct(goFieldI, reflect.New(goFieldValue.Type())); err != nil {
	//		if err := cToGoStruct(goFieldI, goFieldValue.Pointer()); err != nil {
	//			return err
	//		}
	//	default:
	//		fmt.Println("default case!")
	//	}
	//}
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
