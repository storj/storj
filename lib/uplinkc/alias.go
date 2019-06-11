// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

// CPChar exposing C for testing
type CPChar = *C.char
// CUplinkConfig exposing C for testing
type CUplinkConfig = C.UplinkConfig