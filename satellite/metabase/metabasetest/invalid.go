// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// InvalidObjectStream contains info about an invalid stream.
type InvalidObjectStream struct {
	Name         string
	ObjectStream metabase.ObjectStream
	ErrClass     *errs.Class
	ErrText      string
}

// InvalidObjectStreams returns a list of invalid object streams.
func InvalidObjectStreams(base metabase.ObjectStream) []InvalidObjectStream {
	var tests []InvalidObjectStream
	{
		stream := base
		stream.ProjectID = uuid.UUID{}
		tests = append(tests, InvalidObjectStream{
			Name:         "ProjectID missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "ProjectID missing",
		})
	}
	{
		stream := base
		stream.BucketName = ""
		tests = append(tests, InvalidObjectStream{
			Name:         "BucketName missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "BucketName missing",
		})
	}
	{
		stream := base
		stream.ObjectKey = ""
		tests = append(tests, InvalidObjectStream{
			Name:         "ObjectKey missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "ObjectKey missing",
		})
	}
	{
		stream := base
		stream.StreamID = uuid.UUID{}
		tests = append(tests, InvalidObjectStream{
			Name:         "StreamID missing",
			ObjectStream: stream,
			ErrClass:     &metabase.ErrInvalidRequest,
			ErrText:      "StreamID missing",
		})
	}

	return tests
}

// InvalidObjectLocation contains info about an invalid object location.
type InvalidObjectLocation struct {
	Name           string
	ObjectLocation metabase.ObjectLocation
	ErrClass       *errs.Class
	ErrText        string
}

// InvalidObjectLocations returns a list of invalid object locations.
func InvalidObjectLocations(base metabase.ObjectLocation) []InvalidObjectLocation {
	var tests []InvalidObjectLocation
	{
		location := base
		location.ProjectID = uuid.UUID{}
		tests = append(tests, InvalidObjectLocation{
			Name:           "ProjectID missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "ProjectID missing",
		})
	}
	{
		location := base
		location.BucketName = ""
		tests = append(tests, InvalidObjectLocation{
			Name:           "BucketName missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "BucketName missing",
		})
	}
	{
		location := base
		location.ObjectKey = ""
		tests = append(tests, InvalidObjectLocation{
			Name:           "ObjectKey missing",
			ObjectLocation: location,
			ErrClass:       &metabase.ErrInvalidRequest,
			ErrText:        "ObjectKey missing",
		})
	}

	return tests
}

// InvalidSegmentLocation contains info about an invalid segment location.
type InvalidSegmentLocation struct {
	Name            string
	SegmentLocation metabase.SegmentLocation
	ErrClass        *errs.Class
	ErrText         string
}

// InvalidSegmentLocations returns a list of invalid segment locations.
func InvalidSegmentLocations(base metabase.SegmentLocation) []InvalidSegmentLocation {
	var tests []InvalidSegmentLocation
	{
		location := base
		location.ProjectID = uuid.UUID{}
		tests = append(tests, InvalidSegmentLocation{
			Name:            "ProjectID missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "ProjectID missing",
		})
	}
	{
		location := base
		location.BucketName = ""
		tests = append(tests, InvalidSegmentLocation{
			Name:            "BucketName missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "BucketName missing",
		})
	}
	{
		location := base
		location.ObjectKey = ""
		tests = append(tests, InvalidSegmentLocation{
			Name:            "ObjectKey missing",
			SegmentLocation: location,
			ErrClass:        &metabase.ErrInvalidRequest,
			ErrText:         "ObjectKey missing",
		})
	}

	return tests
}
