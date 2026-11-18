// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAPI_Group(t *testing.T) {
	t.Run("valid name and prefix", func(t *testing.T) {
		api := API{}

		require.NotPanics(t, func() {
			api.Group("testName", "tName")
		})

		require.NotPanics(t, func() {
			api.Group("TestName1", "TName1")
		})
	})

	t.Run("invalid name", func(t *testing.T) {
		api := API{}

		require.Panics(t, func() {
			api.Group("1testName", "tName")
		})

		require.Panics(t, func() {
			api.Group("test-name", "tName")
		})
	})

	t.Run("invalid prefix", func(t *testing.T) {
		api := API{}

		require.Panics(t, func() {
			api.Group("testName", "5tName")
		})

		require.Panics(t, func() {
			api.Group("testname", "t_name")
		})
	})

	t.Run("group with repeated name", func(t *testing.T) {
		api := API{}

		require.NotPanics(t, func() {
			api.Group("testName", "tName")
		})

		require.Panics(t, func() {
			api.Group("TESTNAME", "tName2")
		})

		require.Panics(t, func() {
			api.Group("testname", "tName3")
		})
	})

	t.Run("group with repeated prefix", func(t *testing.T) {
		api := API{}

		require.NotPanics(t, func() {
			api.Group("testName", "tName")
		})

		require.Panics(t, func() {
			api.Group("testName2", "tname")
		})

		require.Panics(t, func() {
			api.Group("testname3", "tnamE")
		})
	})
}

func TestParseJSONTag_Omit(t *testing.T) {
	type nested struct{ A int }
	type example struct {
		Plain     bool   `json:"plain"`
		Empty     bool   `json:"empty,omitempty"`
		EmptyStr  nested `json:"empty_struct,omitempty"`
		Zero      bool   `json:"zero,omitzero"`
		ZeroStr   nested `json:"zero_struct,omitzero"`
		EmptyZero nested `json:"empty_zero,omitempty,omitzero"`
	}

	typ := reflect.TypeFor[example]()
	expected := map[string]bool{
		"plain":        false,
		"empty":        true,
		"empty_struct": false, // encoding/json never omits a struct with omitempty
		"zero":         true,
		"zero_struct":  true,
		"empty_zero":   true,
	}
	for i := range typ.NumField() {
		info, ok, err := parseJSONTag(typ, typ.Field(i))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, expected[info.FieldName], info.OmitEmpty, info.FieldName)
	}
}
