// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"regexp"
	"time"
)

type ApplicationFilter struct {
	re *regexp.Regexp
}

func NewApplicationFilter(regex string) *ApplicationFilter {
	return &ApplicationFilter{
		re: regexp.MustCompile(regex),
	}
}

func (a *ApplicationFilter) Filter(application, instance string) bool {
	return a.re.MatchString(application)
}

type InstanceFilter struct {
	re *regexp.Regexp
}

func NewInstanceFilter(regex string) *InstanceFilter {
	return &InstanceFilter{
		re: regexp.MustCompile(regex),
	}
}

func (a *InstanceFilter) Filter(application, instance string) bool {
	return a.re.MatchString(instance)
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
