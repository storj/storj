// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !race

package tagsql

type tracker struct{}

func rootTracker(skip int) *tracker { return nil }

func (t *tracker) child(skip int) *tracker { return nil }
func (t *tracker) close() error            { return nil }
func (t *tracker) formatStack() string     { return "<no start stack for !race>" }
