// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

/*
#include <stdint.h>
#include <stdbool.h>

typedef struct __APIKeyRef_t { long ref; } APIKeyRef_t;
typedef struct __UplinkRef_t { long ref; } UplinkRef_t;
typedef struct __UplinkConfigRef_t { long ref; } UplinkConfigRef_t;
typedef struct __ProjectRef_t { long ref; } ProjectRef_t;

// TODO: Add free functions for each struct

typedef struct Bytes {
	uint8_t *bytes;
	int32_t length;
} Bytes_t;

typedef struct IDVersion {
	uint16_t number;
} IDVersion_t;
*/
import "C"

import (
	"errors"

	"gopkg.in/spacemonkeygo/monkit.v2"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var mon = monkit.Package()

func main() {}

//export GetIDVersion
func GetIDVersion(number C.uint8_t, cerr **C.char) C.IDVersion_t {
	version, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.IDVersion_t{}
	}

	return C.IDVersion_t{
		number: C.uint16_t(version.Number),
	}
}

//export ParseAPIKey
func ParseAPIKey(val *C.char, cerr **C.char) C.APIKeyRef_t {
	apikey, err := libuplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.APIKeyRef_t{}
	}

	return C.APIKeyRef_t{universe.Add(apikey)}
}

//export FreeAPIKey
func FreeAPIKey(apikeyref C.APIKeyRef_t, cerr **C.char) {
	universe.Del(apikeyref.ref)
}

//export SerializeAPIKey
func SerializeAPIKey(cAPIKey C.APIKeyRef_t, cerr **C.char) *C.char {
	apikey, ok := universe.Get(cAPIKey.ref).(libuplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(apikey.Serialize())
}

type Uplink struct {
	scope
	lib *libuplink.Uplink
}

//export NewUplink
func NewUplink(cerr **C.char) C.UplinkRef_t {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.UplinkRef_t{}
	}

	return C.UplinkRef_t{universe.Add(&Uplink{scope, lib})}
}

//export NewUplinkInsecure
func NewUplinkInsecure(cerr **C.char) C.UplinkRef_t {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.UplinkRef_t{}
	}

	return C.UplinkRef_t{universe.Add(&Uplink{scope, lib})}
}

type Project struct {
	scope
	lib *libuplink.Project
}

//export OpenProject
func OpenProject(uplinkref C.UplinkRef_t, satelliteAddr *C.char, cAPIKey C.APIKeyRef_t, cerr **C.char) C.ProjectRef_t {
	uplink, ok := universe.Get(uplinkref.ref).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.ProjectRef_t{}
	}

	var err error
	defer mon.Task()(&uplink.scope.ctx)(&err)

	apikey, ok := universe.Get(cAPIKey.ref).(libuplink.APIKey)
	if !ok {
		err = errors.New("missing API Key")
		*cerr = C.CString(err.Error())
		return C.ProjectRef_t{}
	}

	scope := uplink.scope.child()

	// TODO: add project options argument
	var project *libuplink.Project
	project, err = uplink.lib.OpenProject(scope.ctx, C.GoString(satelliteAddr), apikey, nil)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.ProjectRef_t{}
	}

	return C.ProjectRef_t{universe.Add(&Project{scope, project})}
}

//export CloseProject
func CloseProject(projectref C.ProjectRef_t, cerr **C.char) {
	project, ok := universe.Get(projectref.ref).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(projectref.ref)
	defer project.cancel()

	if err := project.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

//export CloseUplink
func CloseUplink(uplinkref C.UplinkRef_t, cerr **C.char) {
	uplink, ok := universe.Get(uplinkref.ref).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(uplinkref.ref)
	defer uplink.cancel()

	if err := uplink.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

//export internal_UniverseIsEmpty
func internal_UniverseIsEmpty() bool {
	return universe.Empty()
}