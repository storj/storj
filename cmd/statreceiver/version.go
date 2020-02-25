// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"time"
)

// VersionSplit downgrades known v3 metrics into v2 versions for backwards compat.
type VersionSplit struct {
	v2dest MetricDest
	v3dest MetricDest
}

// NewVersionSplit constructs a VersionSplit that passes known v3 metrics as
// v2 metrics to the provided dest.
func NewVersionSplit(v2dest, v3dest MetricDest) *VersionSplit {
	return &VersionSplit{
		v2dest: v2dest,
		v3dest: v3dest,
	}
}

// Metric implements MetricDest
func (k *VersionSplit) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	comma := bytes.IndexByte(key, ',')
	if comma < 0 {
		return k.v2dest.Metric(application, instance, key, val, ts)
	}
	return k.v3dest.Metric(application, instance, key, val, ts)
}
