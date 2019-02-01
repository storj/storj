// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"strings"
	"time"
	"unicode"
)

// Sanitizer is a MetricDest that replaces nonalphanumeric characters with
// underscores.
type Sanitizer struct {
	dest MetricDest
}

// NewSanitizer creates a Sanitizer that sends sanitized metrics to dest.
func NewSanitizer(dest MetricDest) *Sanitizer { return &Sanitizer{dest: dest} }

// Metric implements MetricDest
func (s *Sanitizer) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	return s.dest.Metric(sanitize(application), sanitize(instance), sanitizeb(key), val, ts)
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
