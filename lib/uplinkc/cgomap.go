// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import "sync"

type Metadata struct {
	mtx sync.Mutex
	m   map[string]string
}

// new_metadata returns a new ref/handle to a go map[string]string.
//export new_metadata
func new_metadata() C.MetadataRef {
	metaData := &Metadata{}
	metaData.m = make(map[string]string)
	return C.MetadataRef{universe.Add(metaData)}
}

// metadata_set sets the passed key to the passed value in the go map that the passed ref refers to.
//export metadata_set
func metadata_set(metaDataRef C.MetadataRef, key *C.char, value *C.char, cErr **C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*Metadata)
	if !ok {
		*cErr = C.CString("invalid map")
		return
	}

	metaData.mtx.Lock()
	metaData.m[C.GoString(key)] = C.GoString(value)
	metaData.mtx.Unlock()
}

// metadata_get gets the value of the passed key in the go map that the passed ref refers to.
//export metadata_get
func metadata_get(metaDataRef C.MetadataRef, key *C.char, cErr **C.char) (cValue *C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*Metadata)
	if !ok {
		*cErr = C.CString("invalid map")
		return cValue
	}

	metaData.mtx.Lock()
	value := metaData.m[C.GoString(key)]
	metaData.mtx.Unlock()

	return C.CString(value)
}

// metadata_del deletes value of the passed key in the go map that the passed ref refers to.
//export metadata_del
func metadata_del(metaDataRef C.MetadataRef, key *C.char, cErr **C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*Metadata)
	if !ok {
		*cErr = C.CString("invalid map")
	}

	metaData.mtx.Lock()
	delete(metaData.m, C.GoString(key))
	metaData.mtx.Unlock()
}

// free_metadata deletes a ref/handle to a go map[string]string.
//export free_metadata
func free_metadata(metaDataRef C.MetadataRef) {
	universe.Del(metaDataRef._handle)
}
