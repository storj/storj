// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

/*
#include <stdint.h>
#include <stdbool.h>

typedef struct APIKey       { long _ref; } APIKey;
typedef struct Uplink       { long _ref; } Uplink;
typedef struct Project      { long _ref; } Project;
*/
import "C"

import (
	"gopkg.in/spacemonkeygo/monkit.v2"

	libuplink "storj.io/storj/lib/uplink"
	// "storj.io/storj/pkg/storj"
)

var mon = monkit.Package()

func main() {}

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
func OpenProject(uplinkref C.Uplink, satelliteAddr *C.char, apikeystr *C.char, cerr **C.char) C.Project {
	uplink, ok := universe.Get(uplinkref._ref).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.Project{}
	}

	var err error
	defer mon.Task()(&uplink.scope.ctx)(&err)

	apikey, err := libuplink.ParseAPIKey(C.GoString(apikeystr))
	if err != nil {
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