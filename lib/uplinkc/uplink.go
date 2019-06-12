// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import libuplink "storj.io/storj/lib/uplink"

// Uplink is a scoped libuplink.Uplink.
type Uplink struct {
	scope
	lib *libuplink.Uplink
}

//export new_uplink
// new_uplink creates the uplink with the specified configuration and returns
// an error in cerr, when there is one.
//
// Caller must call close_uplink to close associated resources.
func new_uplink(cfg C.UplinkConfig, cerr **C.char) C.Uplink {
	scope := rootScope("inmemory") // TODO: pass in as argument

	libcfg := &libuplink.Config{} // TODO: figure out a better name
	libcfg.Volatile.TLS.SkipPeerCAWhitelist = cfg.Volatile.TLS.SkipPeerCAWhitelist == 1

	lib, err := libuplink.NewUplink(scope.ctx, libcfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Uplink{}
	}

	return C.Uplink{universe.Add(&Uplink{scope, lib})}
}

//export close_uplink
// close_uplink closes and frees the resources associated with uplink
func close_uplink(uplinkHandle C.Uplink, cerr **C.char) {
	uplink, ok := universe.Get(uplinkHandle._handle).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(uplinkHandle._handle)
	defer uplink.cancel()

	if err := uplink.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

//export open_project
// open_project opens project using uplink
func open_project(uplinkHandle C.Uplink, satelliteAddr *C.char, apikeyHandle C.APIKey, cerr **C.char) C.Project {
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
