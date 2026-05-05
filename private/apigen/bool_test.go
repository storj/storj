// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateGo_BoolQueryParam(t *testing.T) {
	a := &API{
		PackagePath: "storj.io/storj/private/apigen/testbool",
		Version:     "v1",
		BasePath:    "/api",
	}

	g := a.Group("Test", "test")
	g.Get("/required", &Endpoint{
		Name:           "Required Bool",
		Description:    "Endpoint with a required bool query param",
		GoName:         "RequiredBool",
		TypeScriptName: "requiredBool",
		QueryParams: []QueryParam{
			NewQueryParam("active", false),
		},
	})
	g.Get("/optional-static", &Endpoint{
		Name:           "Optional Static Bool",
		Description:    "Endpoint with a static optional bool query param",
		GoName:         "OptionalStaticBool",
		TypeScriptName: "optionalStaticBool",
		QueryParams: []QueryParam{
			NewQueryParamOptional("active", true),
		},
	})
	g.Get("/optional-dynamic", &Endpoint{
		Name:           "Optional Dynamic Bool",
		Description:    "Endpoint with a dynamic optional bool query param",
		GoName:         "OptionalDynamicBool",
		TypeScriptName: "optionalDynamicBool",
		QueryParams: []QueryParam{
			NewQueryParamOptionalDynamic("active", func() interface{} { return false }),
		},
	})

	code, err := a.generateGo()
	require.NoError(t, err)

	src := string(code)

	// Required bool: empty check + ParseBool.
	assert.Contains(t, src, `"parameter 'active' can't be empty"`)
	assert.Contains(t, src, `strconv.ParseBool`)

	// Static optional bool: Has check + ParseBool + static field assignment.
	// The formatter aligns struct fields so we match substrings without exact spacing.
	assert.Contains(t, src, `defaultOptionalStaticBoolActive`)
	assert.Contains(t, src, `defaultOptionalStaticBoolActive:`)
	assert.Contains(t, src, `true,`)

	// Dynamic optional bool: Has check + ParseBool + type assert.
	assert.Contains(t, src, `func() interface{}`)
	assert.Contains(t, src, `.(bool)`)

	// No unsupported type errors.
	assert.NotContains(t, src, "Unsupported")

	// Verify strconv is imported.
	assert.True(t, strings.Contains(src, `"strconv"`))
}
