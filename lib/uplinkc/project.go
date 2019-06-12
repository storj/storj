// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"storj.io/storj/lib/uplink"
)

// Project is a scoped uplink.Project
type Project struct {
	scope
	*uplink.Project
}

//export open_project
// open_project opens project using uplink
func open_project(uplinkHandle C.Uplink, satelliteAddr *C.char, apikeyHandle C.APIKey, cerr **C.char) C.Project {
	up, ok := universe.Get(uplinkHandle._handle).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.Project{}
	}

	apikey, ok := universe.Get(apikeyHandle._handle).(uplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
		return C.Project{}
	}

	scope := up.scope.child()

	// TODO: add project options argument
	project, err := up.OpenProject(scope.ctx, C.GoString(satelliteAddr), apikey, nil)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Project{}
	}

	return C.Project{universe.Add(&Project{scope, project})}
}

//export close_project
// close_project closes the project.
func close_project(projectHandle C.Project, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(projectHandle._handle)
	defer project.cancel()

	if err := project.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}
