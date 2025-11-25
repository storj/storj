// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hotel

import "strings"

// SeriesKey represents an individual time series for monkit to output.
type SeriesKey struct {
	Measurement string
	Tags        *TagSet
}

// NewSeriesKey constructs a new series with the minimal fields.
func NewSeriesKey(measurement string) SeriesKey {
	return SeriesKey{Measurement: measurement}
}

// WithTag returns a copy of the SeriesKey with the tag set
func (s SeriesKey) WithTag(key, value string) SeriesKey {
	s.Tags = s.Tags.Set(key, value)
	return s
}

// WithTags returns a copy of the SeriesKey with all of the tags set
func (s SeriesKey) WithTags(tags ...SeriesTag) SeriesKey {
	s.Tags = s.Tags.SetTags(tags...)
	return s
}

// String returns a string representation of the series. For example, it returns
// something like `measurement,tag0=val0,tag1=val1`.
func (s SeriesKey) String() string {
	var builder strings.Builder
	writeMeasurement(&builder, s.Measurement)
	if s.Tags.Len() > 0 {
		builder.WriteByte(',')
		builder.WriteString(s.Tags.String())
	}
	return builder.String()
}

// WithField returns a string representation of the series key with the field name appended.
func (s SeriesKey) WithField(field string) string {
	var builder strings.Builder
	builder.WriteString(s.String())
	builder.WriteByte(' ')
	writeTag(&builder, field)
	return builder.String()
}
