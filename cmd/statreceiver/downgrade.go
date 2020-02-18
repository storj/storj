// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"time"
)

// Note to readers: All of the tag iteration and field searching code does not bother to
// handle escaped spaces or commas because all of the keys we intend to migrate do not
// contain them. In particular, we have no package import paths with spaces or commas and
// it is impossible to have a function name with a space or comma in it. Additionally, all
// of the monkit/v3 field names do not have spaces or commas. This could become invalid if
// someone decides to break it, but this code is also temporary.

// MetricDowngrade downgrades known v3 metrics into v2 versions for backwards compat.
type MetricDowngrade struct {
	dest MetricDest
}

// knownMetrics is decoded in main.go using a provided toml configuration file.
var knownMetrics map[string]string

// NewMetricDowngrade constructs a MetricDowngrade that passes known v3 metrics as
// v2 metrics to the provided dest.
func NewMetricDowngrade(dest MetricDest) *MetricDowngrade {
	return &MetricDowngrade{
		dest: dest,
	}
}

// Metric implements MetricDest
func (k *MetricDowngrade) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	comma := bytes.IndexByte(key, ',')
	if comma < 0 {
		return nil
	}

	if string(key[:comma]) == "function_times" {
		return k.handleFunctionTimes(application, instance, key[comma+1:], val, ts)
	}
	if string(key[:comma]) == "function" {
		return k.handleFunction(application, instance, key[comma+1:], val, ts)
	}

	v2key, ok := knownMetrics[string(key[:comma])]
	if !ok {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(v2key)+1+len(key)-space)
	out = append(out, v2key...)
	out = append(out, '.')
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func (k *MetricDowngrade) handleFunctionTimes(application, instance string, key []byte, val float64, ts time.Time) error {
	var name, kind, scope string
	iterateTags(key, func(tag []byte) {
		if len(tag) < 6 {
			return
		}
		switch {
		case string(tag[:5]) == "name=":
			name = string(tag[5:])
		case string(tag[:5]) == "kind=":
			kind = string(tag[5:])
		case string(tag[:6]) == "scope=":
			scope = string(tag[6:])
		}
	})

	if name == "" || kind == "" || scope == "" {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(scope)+1+len(name)+1+len(kind)+7+(len(key)-space))
	out = append(out, scope...)
	out = append(out, '.')
	out = append(out, name...)
	out = append(out, '.')
	out = append(out, kind...)
	out = append(out, "_times_"...)
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func (k *MetricDowngrade) handleFunction(application, instance string, key []byte, val float64, ts time.Time) error {
	var name, scope string
	iterateTags(key, func(tag []byte) {
		if len(tag) < 6 {
			return
		}
		switch {
		case string(tag[:5]) == "name=":
			name = string(tag[5:])
		case string(tag[:6]) == "scope=":
			scope = string(tag[6:])
		}
	})

	if name == "" || scope == "" {
		return nil
	}

	space := bytes.LastIndexByte(key, ' ')
	if space < 0 {
		return nil
	}

	out := make([]byte, 0, len(scope)+1+len(name)+1+(len(key)-space))
	out = append(out, scope...)
	out = append(out, '.')
	out = append(out, name...)
	out = append(out, '.')
	out = append(out, key[space+1:]...)

	return k.dest.Metric(application, instance, out, val, ts)
}

func iterateTags(key []byte, cb func([]byte)) {
	for len(key) > 0 {
		comma := bytes.IndexByte(key, ',')
		if comma == -1 {
			break
		}
		cb(key[:comma])
		key = key[comma+1:]
	}
	space := bytes.IndexByte(key, ' ')
	if space >= 0 {
		cb(key[:space])
	}
}
