// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "C"
import (
	"context"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/lib/uplink"
)

var mon = monkit.Package()

//export NewUplink
func NewUplink(cErr *CCharPtr) (cUplink CUplinkRef) {
	goUplink, err := uplink.NewUplink(context.Background(), &uplink.Config{})
	if err != nil {
		*cErr = CCString(err.Error())
		return cUplink
	}

	return CUplinkRef(universe.Add(goUplink))
}

//export NewUplinkInsecure
func NewUplinkInsecure(cErr *CCharPtr) (cUplink CUplinkRef) {
	insecureConfig := &uplink.Config{}
	insecureConfig.Volatile.TLS.SkipPeerCAWhitelist = true
	goUplink, err := uplink.NewUplink(context.Background(), insecureConfig)
	if err != nil {
		*cErr = CCString(err.Error())
		return cUplink
	}

	return CUplinkRef(universe.Add(goUplink))
}

//export OpenProject
func OpenProject(cUplink CUplinkRef, satelliteAddr CCharPtr, cAPIKey CAPIKeyRef, cErr *CCharPtr) (cProject CProjectRef) {
	var err error
	ctx := context.Background()
	defer mon.Task()(&ctx)(&err)

	goUplink, ok := universe.Get(Token(cUplink)).(*uplink.Uplink)
	if !ok {
		*cErr = CCString("invalid uplink")
		return cProject
	}

	apiKey, ok := universe.Get(Token(cAPIKey)).(uplink.APIKey)
	if !ok {
		*cErr = CCString("invalid API Key")
		return cProject
	}

	// TODO: add project options argument
	project, err := goUplink.OpenProject(ctx, CGoString(satelliteAddr), apiKey, nil)
	if err != nil {
		*cErr = CCString(err.Error())
		return cProject
	}
	return CProjectRef(universe.Add(project))
}

//export CloseUplink
func CloseUplink(cUplink CUplinkRef, cErr *CCharPtr) {
	goUplink, ok := universe.Get(Token(cUplink)).(*uplink.Uplink)
	if !ok {
		*cErr = CCString("invalid uplink")
		return
	}

	if err := goUplink.Close(); err != nil {
		*cErr = CCString(err.Error())
		return
	}
}
