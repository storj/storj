// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go build -o uplink-cgo-common.so -buildmode=c-shared .

package main

// #cgo CFLAGS: -ggdb -Wall
import "C"
import (
	"context"

	"storj.io/storj/lib/uplink"
)

func main() {}

//export NewUplink
func NewUplink(cConfig C.struct_Config, cErr *C.char) C.struct_Config {
	//goConfig, err := cToGoStruct(cConfig)
	goConfig := uplink.Config{}
	goConfig.Volatile.TLS.SkipPeerCAWhitelist = true
	//if err != nil {
	//
	//}

	_, err := uplink.NewUplink(context.Background(), &goConfig)
	if err != nil {
		*cErr = *C.CString(err.Error())
	}

	//return C.struct_Uplink{
	//	GoUplink: goUplink,
	//	config: cConfig,
	//}
	return cConfig
	//fmt.Printf("go: %s\n", cUplink.volatile_.tls.SkipPeerCAWhitelist)
}
