// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"context"
	"unsafe"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// Project is a scoped libuplink.Project
type Project struct {
	scope
	lib *libuplink.Project
}

//export OpenProject
// OpenProject opens project using uplink
func OpenProject(uplinkHandle C.Uplink, satelliteAddr *C.char, apikeyHandle C.APIKey, cerr **C.char) C.Project {
	uplink, ok := universe.Get(uplinkHandle._handle).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.Project{}
	}

	var err error

	apikey, ok := universe.Get(apikeyHandle._handle).(libuplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
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
// CloseProject closes the project.
func CloseProject(projectHandle C.Project, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(projectHandle._handle)
	defer project.cancel()

	if err := project.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}