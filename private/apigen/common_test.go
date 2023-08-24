// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_endpointBasePath(t *testing.T) {
	cases := []struct {
		version  string
		basePath string
		expected string
	}{
		{version: "", basePath: "", expected: "/"},
		{version: "v1", basePath: "", expected: "/v1"},
		{version: "v0", basePath: "/", expected: "/v0"},
		{version: "", basePath: "api", expected: "/api"},
		{version: "v2", basePath: "api", expected: "/api/v2"},
		{version: "v2", basePath: "/api", expected: "/api/v2"},
		{version: "v2", basePath: "api/", expected: "/api/v2"},
		{version: "v2", basePath: "/api/", expected: "/api/v2"},
		{version: "/v3", basePath: "api", expected: "/api/v3"},
		{version: "/v3/", basePath: "api", expected: "/api/v3"},
		{version: "v3/", basePath: "api", expected: "/api/v3"},
		{version: "//v3/", basePath: "api", expected: "/api/v3"},
		{version: "v3///", basePath: "api", expected: "/api/v3"},
		{version: "/v3///", basePath: "/api/test/", expected: "/api/test/v3"},
		{version: "/v4.2", basePath: "api/test", expected: "/api/test/v4.2"},
		{version: "/v4/2", basePath: "/api/test", expected: "/api/test/v4/2"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("version:%s basePath: %s", c.version, c.basePath), func(t *testing.T) {
			a := API{
				Version:  c.version,
				BasePath: c.basePath,
			}

			assert.Equal(t, c.expected, a.endpointBasePath())
		})
	}
}
