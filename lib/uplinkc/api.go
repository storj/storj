// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

/*
#include <stdint.h>
#include <stdbool.h>

typedef struct APIKey { long _ref; } APIKey;
typedef struct Uplink { long _ref; } Uplink;
typedef struct UplinkConfig { long _ref; } UplinkConfig;
typedef struct Project { long _ref; } Project;

// TODO: Add free functions for each struct

typedef struct Bytes {
	uint8_t *bytes;
	int32_t length;
} Bytes;

typedef struct IDVersion {
	uint16_t number;
} IDVersion;
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
func GetIDVersion(number C.uint8_t, cerr **C.char) C.IDVersion {
	version, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.IDVersion{}
	}

	return C.IDVersion{
		number: C.uint16_t(version.Number),
	}
}

//export ParseAPIKey
func ParseAPIKey(val *C.char, cerr **C.char) C.APIKey {
	apikey, err := libuplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.APIKey{}
	}

	return C.APIKey{universe.Add(apikey)}
}

//export FreeAPIKey
func FreeAPIKey(apikeyref C.APIKey, cerr **C.char) {
	universe.Del(apikeyref._ref)
}

//export SerializeAPIKey
func SerializeAPIKey(cAPIKey C.APIKey, cerr **C.char) *C.char {
	apikey, ok := universe.Get(cAPIKey._ref).(libuplink.APIKey)
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
func NewUplink(cerr **C.char) C.Uplink {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Uplink{}
	}

	return C.Uplink{universe.Add(&Uplink{scope, lib})}
}

//export NewUplinkInsecure
func NewUplinkInsecure(cerr **C.char) C.Uplink {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Uplink{}
	}

	return C.Uplink{universe.Add(&Uplink{scope, lib})}
}

type Project struct {
	scope
	lib *libuplink.Project
}

//export OpenProject
func OpenProject(uplinkref C.Uplink, satelliteAddr *C.char, cAPIKey C.APIKey, cerr **C.char) C.Project {
	uplink, ok := universe.Get(uplinkref._ref).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.Project{}
	}

	var err error
	defer mon.Task()(&uplink.scope.ctx)(&err)

	apikey, ok := universe.Get(cAPIKey._ref).(libuplink.APIKey)
	if !ok {
		err = errors.New("missing API Key")
		*cerr = C.CString(err.Error())
		return C.Project{}
	}

	scope := uplink.scope.child()

	// TODO: add project options argument
	var project *libuplink.Project
	project, err = uplink.lib.OpenProject(scope.ctx, C.GoString(satelliteAddr), apikey, nil)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Project{}
	}

	return C.Project{universe.Add(&Project{scope, project})}
}

//export CloseProject
func CloseProject(projectref C.Project, cerr **C.char) {
	project, ok := universe.Get(projectref._ref).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(projectref._ref)
	defer project.cancel()

	if err := project.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

//export CloseUplink
func CloseUplink(uplinkref C.Uplink, cerr **C.char) {
	uplink, ok := universe.Get(uplinkref._ref).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(uplinkref._ref)
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