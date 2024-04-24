// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux && !darwin && !freebsd

package main

func raiseUlimits() {}
