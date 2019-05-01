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
	"fmt"
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
	if err := goToCStruct(goIDVersion, cIDVersion); err != nil {
		*cErr = *C.CString(err.Error())
		return cIDVersion
	}
	return cIDVersion
}

func goToCStruct(goStruct interface{}, cPtr interface{}) error {
	goStructValue := reflect.ValueOf(goStruct)
	switch goStructValue.Kind() {
	case reflect.Uintptr:
		fmt.Println("Uintptr case!")
		// NB: `goStruct` is a pointer;
		value := reflect.Indirect(goStructValue)
		goToCStruct(value.Interface(), cPtr)
	case reflect.Struct:
		fmt.Println("outer struct case!")
		goFieldI := goStructValue.Interface()
		fmt.Printf("%+v\n", goFieldI)
	default:
		fmt.Println("outer default case!")
		//reflect.Indirect(reflect.ValueOf(cPtr))
		reflect.Indirect(reflect.ValueOf(cPtr))
		v := reflect.ValueOf(cPtr)
		v.Pointer()
	}

	//for i := 0; i < goStructValue.NumField(); i++ {
	//	field := goStructValue.Field(i)
	//	fmt.Printf("%+v\n", field)
	//
	//	//fmt.Printf("%s\n", field.Name)
	//	goPtrValue := reflect.ValueOf(cPtr)
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
