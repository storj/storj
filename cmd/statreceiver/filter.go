// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"regexp"
	"time"
)

type PacketFilter struct {
	appRE, instRE *regexp.Regexp
}

func NewPacketFilter(applicationRE, instanceRE string) *PacketFilter {
	return &PacketFilter{
		appRE:  regexp.MustCompile(applicationRE),
		instRE: regexp.MustCompile(instanceRE),
	}
}

func (a *PacketFilter) Filter(application, instance string) bool {
	return a.appRE.MatchString(application) && a.instRE.MatchString(instance)
}

type KeyFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

func NewKeyFilter(regex string, m MetricDest) *KeyFilter {
	return &KeyFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

func (k *KeyFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.Match(key) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}

type ApplicationFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

func NewApplicationFilter(regex string, m MetricDest) *ApplicationFilter {
	return &ApplicationFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

func (k *ApplicationFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.MatchString(application) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}

type InstanceFilter struct {
	re *regexp.Regexp
	m  MetricDest
}

func NewInstanceFilter(regex string, m MetricDest) *InstanceFilter {
	return &InstanceFilter{
		re: regexp.MustCompile(regex),
		m:  m,
	}
}

func (k *InstanceFilter) Metric(application, instance string,
	key []byte, val float64, ts time.Time) error {
	if k.re.MatchString(instance) {
		return k.m.Metric(application, instance, key, val, ts)
	}
	return nil
}
