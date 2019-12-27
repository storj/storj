// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"regexp"
	"sync"
	"time"

	"github.com/zeebo/admission/admproto"

	"storj.io/common/memory"
)

// PacketFilter inspects a packet header to determine if it should be passed
// through
type PacketFilter struct {
	application *regexp.Regexp
	instance    *regexp.Regexp
	dest        PacketDest
	scratch     sync.Pool
}

// NewPacketFilter creates a PacketFilter. It takes a packet destination,
// an application regular expression, and an instance regular expression.
// If the regular expression is matched, the packet will be passed through.
func NewPacketFilter(applicationRegex, instanceRegex string, dest PacketDest) *PacketFilter {
	return &PacketFilter{
		application: regexp.MustCompile(applicationRegex),
		instance:    regexp.MustCompile(instanceRegex),
		dest:        dest,
		scratch: sync.Pool{
			New: func() interface{} {
				var x [10 * memory.KB]byte
				return &x
			},
		},
	}
}

// Packet passes the packet along to the given destination if the regexes pass
func (a *PacketFilter) Packet(data []byte, ts time.Time) error {
	cdata, err := admproto.CheckChecksum(data)
	if err != nil {
		return err
	}
	scratch := a.scratch.Get().(*[10 * memory.KB]byte)
	defer a.scratch.Put(scratch)

	r := admproto.NewReaderWith((*scratch)[:])
	_, application, instance, err := r.Begin(cdata)
	if err != nil {
		return err
	}
	if a.application.Match(application) && a.instance.Match(instance) {
		return a.dest.Packet(data, ts)
	}
	return nil
}

// KeyFilter is a MetricDest that only passes along metrics that pass the key
// filter
type KeyFilter struct {
	pattern *regexp.Regexp
	dest    MetricDest
}

// NewKeyFilter creates a KeyFilter. pattern is the regular expression that must
// match, and dest is the MetricDest to send matching metrics to.
func NewKeyFilter(pattern string, dest MetricDest) *KeyFilter {
	return &KeyFilter{
		pattern: regexp.MustCompile(pattern),
		dest:    dest,
	}
}

// Metric implements MetricDest
func (k *KeyFilter) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	if k.pattern.Match(key) {
		return k.dest.Metric(application, instance, key, val, ts)
	}
	return nil
}

// ApplicationFilter is a MetricDest that only passes along metrics that pass
// the application filter
type ApplicationFilter struct {
	pattern *regexp.Regexp
	dest    MetricDest
}

// NewApplicationFilter creates an ApplicationFilter. pattern is the regular
// expression that must match, and dest is the MetricDest to send matching metrics
// to.
func NewApplicationFilter(regex string, dest MetricDest) *ApplicationFilter {
	return &ApplicationFilter{
		pattern: regexp.MustCompile(regex),
		dest:    dest,
	}
}

// Metric implements MetricDest
func (k *ApplicationFilter) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	if k.pattern.MatchString(application) {
		return k.dest.Metric(application, instance, key, val, ts)
	}
	return nil
}

// InstanceFilter is a MetricDest that only passes along metrics that pass
// the instance filter
type InstanceFilter struct {
	pattern *regexp.Regexp
	dest    MetricDest
}

// NewInstanceFilter creates an InstanceFilter. pattern is the regular
// expression that must match, and dest is the MetricDest to send matching metrics
// to.
func NewInstanceFilter(regex string, dest MetricDest) *InstanceFilter {
	return &InstanceFilter{
		pattern: regexp.MustCompile(regex),
		dest:    dest,
	}
}

// Metric implements MetricDest
func (k *InstanceFilter) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	if k.pattern.MatchString(instance) {
		return k.dest.Metric(application, instance, key, val, ts)
	}
	return nil
}
