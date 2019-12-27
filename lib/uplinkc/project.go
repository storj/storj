// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"
import (
	"fmt"

	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

// Project is a scoped uplink.Project
type Project struct {
	scope
	*libuplink.Project
}

//export open_project
// open_project opens project using uplink
func open_project(uplinkHandle C.UplinkRef, satelliteAddr *C.char, apikeyHandle C.APIKeyRef, cerr **C.char) C.ProjectRef {
	uplink, ok := universe.Get(uplinkHandle._handle).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.ProjectRef{}
	}

	apikey, ok := universe.Get(apikeyHandle._handle).(libuplink.APIKey)
	if !ok {
		*cerr = C.CString("invalid apikey")
		return C.ProjectRef{}
	}

	scope := uplink.scope.child()

	project, err := uplink.OpenProject(scope.ctx, C.GoString(satelliteAddr), apikey)
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return C.ProjectRef{}
	}

	return C.ProjectRef{universe.Add(&Project{scope, project})}
}

//export project_salted_key_from_passphrase
// project_salted_key_from_passphrase returns a key generated from the given passphrase
// using a stable, project-specific salt
func project_salted_key_from_passphrase(projectHandle C.ProjectRef, passphrase *C.char, cerr **C.char) *C.uint8_t {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return nil
	}

	saltedKey, err := project.SaltedKeyFromPassphrase(project.ctx, C.GoString(passphrase))
	if err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return nil
	}

	ptr := C.malloc(storj.KeySize)
	key := (*storj.Key)(ptr)
	copy(key[:], saltedKey[:])
	return (*C.uint8_t)(ptr)
}

//export close_project
// close_project closes the project.
func close_project(projectHandle C.ProjectRef, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return
	}
	universe.Del(projectHandle._handle)
	defer project.cancel()

	if err := project.Close(); err != nil {
		*cerr = C.CString(fmt.Sprintf("%+v", err))
		return
	}
}
