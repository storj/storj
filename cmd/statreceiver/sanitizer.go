// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"strings"
	"time"
	"unicode"
)

type Sanitizer struct {
	m MetricDest
}

func NewSanitizer(m MetricDest) *Sanitizer { return &Sanitizer{m: m} }

func (s *Sanitizer) Metric(application, instance string, key []byte,
	val float64, ts time.Time) error {
	return s.m.Metric(sanitize(application), sanitize(instance), sanitizeb(key),
		val, ts)
}

func sanitize(val string) string {
	return strings.Replace(strings.Map(safechar, val), "..", ".", -1)
}

func sanitizeb(val []byte) []byte {
	return bytes.Replace(bytes.Map(safechar, val), []byte(".."), []byte("."), -1)
}

func safechar(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsNumber(r) {
		return r
	}
	switch r {
	case '/':
		return '.'
	case '.', '-':
		return r
	}
	return '_'
}
