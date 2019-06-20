// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import "sync"

type MapRef struct {
	mtx sync.Mutex
	m   map[string]string
}

// new_map_ref returns a new ref/handle to a go map[string]string.
//export new_map_ref
func new_map_ref() C.MapRef {
	mapref := &MapRef{}
	mapref.m = make(map[string]string)
	return C.MapRef{universe.Add(mapref)}
}

// map_ref_set sets the passed key to the passed value in the go map that the passed ref refers to.
//export map_ref_set
func map_ref_set(metaDataRef C.MapRef, key *C.char, value *C.char, cErr **C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
		return
	}

	metaData.mtx.Lock()
	metaData.m[C.GoString(key)] = C.GoString(value)
	metaData.mtx.Unlock()
}

// map_ref_get gets the value of the passed key in the go map that the passed ref refers to.
//export map_ref_get
func map_ref_get(metaDataRef C.MapRef, key *C.char, cErr **C.char) (cValue *C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
		return cValue
	}

	metaData.mtx.Lock()
	value := metaData.m[C.GoString(key)]
	metaData.mtx.Unlock()

	return C.CString(value)
}

// map_ref_del deletes value of the passed key in the go map that the passed ref refers to.
//export map_ref_del
func map_ref_del(metaDataRef C.MapRef, key *C.char, cErr **C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
	}

	metaData.mtx.Lock()
	delete(metaData.m, C.GoString(key))
	metaData.mtx.Unlock()
}

// delete_map_ref deletes a ref/handle to a go map[string]string.
//export delete_map_ref
func delete_map_ref(metaDataRef C.MapRef) {
	universe.Del(metaDataRef._handle)
}
