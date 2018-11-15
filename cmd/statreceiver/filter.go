// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"regexp"
	"time"
)

// PacketFilter is used during Packet parsing to determine if the Packet should
// continue to be parsed.
type PacketFilter struct {
	appRE, instRE *regexp.Regexp
}

// NewPacketFilter creates a PacketFilter. It takes an application regular
// expression and an instance regular expression. If the regular expression
// is matched, the packet will be parsed.
func NewPacketFilter(applicationRE, instanceRE string) *PacketFilter {
	return &PacketFilter{
		appRE:  regexp.MustCompile(applicationRE),
		instRE: regexp.MustCompile(instanceRE),
	}
}

// Filter returns true if the application and instance match the filter.
func (a *PacketFilter) Filter(application, instance string) bool {
	return a.appRE.MatchString(application) && a.instRE.MatchString(instance)
}

// KeyFilter is a MetricDest that only passes along metrics that pass the key
// filter
type KeyFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

// NewKeyFilter creates a KeyFilter. regex is the regular expression that must
// match, and m is the MetricDest to send matching metrics to.
func NewKeyFilter(regex string, m MetricDest) *KeyFilter {
	return &KeyFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

// Metric implements MetricDest
func (k *KeyFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.Match(key) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}

// ApplicationFilter is a MetricDest that only passes along metrics that pass
// the application filter
type ApplicationFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

// NewApplicationFilter creates an ApplicationFilter. regex is the regular
// expression that must match, and m is the MetricDest to send matching metrics
// to.
func NewApplicationFilter(regex string, m MetricDest) *ApplicationFilter {
	return &ApplicationFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

// Metric implements MetricDest
func (k *ApplicationFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.MatchString(application) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}

// InstanceFilter is a MetricDest that only passes along metrics that pass
// the instance filter
type InstanceFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

// NewInstanceFilter creates an InstanceFilter. regex is the regular
// expression that must match, and m is the MetricDest to send matching metrics
// to.
func NewInstanceFilter(regex string, m MetricDest) *InstanceFilter {
	return &InstanceFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

// Metric implements MetricDest
func (k *InstanceFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.MatchString(instance) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}
