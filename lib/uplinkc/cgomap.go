package main

// #include "uplink_definitions.h"
import "C"
import "sync"

type MapRef struct {
	mtx sync.Mutex
	m map[string]string
}

// NewMapRef returns a new ref/handle to a go map[string]string.
//export NewMapRef
func NewMapRef() C.MapRef_t {
	mapref := &MapRef{}
	mapref.m = make(map[string]string)
	return C.MapRef_t{universe.Add(mapref)}
}

// MapRefSet sets the passed key to the passed value in the go map that the passed ref refers to.
//export MapRefSet
func MapRefSet(metaDataRef C.MapRef_t, key *C.char, value *C.char, cErr **C.char) {
	metaData, ok := universe.Get(metaDataRef._handle).(*MapRef)
	if !ok {
		*cErr = C.CString("invalid map")
		return
	}

	metaData.mtx.Lock()
	metaData.m[C.GoString(key)] = C.GoString(value)
	metaData.mtx.Unlock()
}

// MapRefGet gets the value of the passed key in the go map that the passed ref refers to.
//export MapRefGet
func MapRefGet(metaDataRef C.MapRef_t, key *C.char, cErr **C.char) (cValue *C.char) {
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
