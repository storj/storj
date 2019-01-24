// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"regexp"
	"time"
)

// PacketFilter is used during Packet parsing to determine if the Packet should
// continue to be parsed.
type PacketFilter struct {
	application *regexp.Regexp
	instance    *regexp.Regexp
}

// NewPacketFilter creates a PacketFilter. It takes an application regular
// expression and an instance regular expression. If the regular expression
// is matched, the packet will be parsed.
func NewPacketFilter(applicationRegex, instanceRegex string) *PacketFilter {
	return &PacketFilter{
		application: regexp.MustCompile(applicationRegex),
		instance:    regexp.MustCompile(instanceRegex),
	}
}

// Filter returns true if the application and instance match the filter.
func (a *PacketFilter) Filter(application, instance string) bool {
	return a.application.MatchString(application) && a.instance.MatchString(instance)
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
