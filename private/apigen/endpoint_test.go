// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"fmt"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpoint_Validate(t *testing.T) {
	validEndpoint := Endpoint{
		Name:           "Test Endpoint",
		Description:    "This is an Endpoint purely for testing purposes",
		GoName:         "GenTest",
		TypeScriptName: "genTest",
	}

	tcases := []struct {
		testName   string
		endpointFn func() *Endpoint
		errMsg     string
	}{
		{
			testName: "valid endpoint",
			endpointFn: func() *Endpoint {
				return &validEndpoint
			},
		},
		{
			testName: "empty name",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.Name = ""
				return &e
			},
			errMsg: "Name cannot be empty",
		},
		{
			testName: "empty description",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.Description = ""
				return &e
			},
			errMsg: "Description cannot be empty",
		},
		{
			testName: "empty Go name",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.GoName = ""
				return &e
			},
			errMsg: "GoName doesn't match the regular expression",
		},
		{
			testName: "no capitalized Go name ",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.GoName = "genTest"
				return &e
			},
			errMsg: "GoName doesn't match the regular expression",
		},
		{
			testName: "symbol in Go name",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.GoName = "GenTe$t"
				return &e
			},
			errMsg: "GoName doesn't match the regular expression",
		},
		{
			testName: "empty TypeScript name",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.TypeScriptName = ""
				return &e
			},
			errMsg: "TypeScriptName doesn't match the regular expression",
		},
		{
			testName: "capitalized TypeScript name ",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.TypeScriptName = "GenTest"
				return &e
			},
			errMsg: "TypeScriptName doesn't match the regular expression",
		},
		{
			testName: "dash in TypeScript name",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.TypeScriptName = "genTest-2"
				return &e
			},
			errMsg: "TypeScriptName doesn't match the regular expression",
		},
		{
			testName: "invalid Request type",
			endpointFn: func() *Endpoint {
				request := &struct {
					Name string `json:"name"`
				}{}
				e := validEndpoint
				e.Request = request
				return &e
			},
			errMsg: fmt.Sprintf("Request cannot be of a type %q", reflect.Pointer),
		},
		{
			testName: "invalid Response type",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.Response = map[string]string{}
				return &e
			},
			errMsg: fmt.Sprintf("Response cannot be of a type %q", reflect.Map),
		},
		{
			testName: "different ResponseMock type",
			endpointFn: func() *Endpoint {
				e := validEndpoint
				e.Response = int(0)
				e.ResponseMock = int8(0)
				return &e
			},
			errMsg: fmt.Sprintf(
				"ResponseMock isn't of the same type than Response. Have=%q Want=%q",
				reflect.TypeOf(int8(0)),
				reflect.TypeOf(int(0)),
			),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.testName, func(t *testing.T) {
			ep := tc.endpointFn()

			err := ep.Validate()

			if tc.errMsg == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestEndpointGroup(t *testing.T) {
	t.Run("add endpoints", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "Test Endpoint",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest" + postfix,
				TypeScriptName: "genTest" + postfix,
			}
		}

		path := "/" + strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.NotPanics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.NotPanics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.NotPanics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.NotPanics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.NotPanics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")

		require.Len(t, eg.endpoints, 5, "Group endpoints count")
		for i, m := range []string{http.MethodGet, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodDelete} {
			ep := eg.endpoints[i]
			assert.Equal(t, m, ep.Method)
			assert.Equal(t, path, ep.Path)
			assert.EqualValues(t, endpointFn(m), &ep.Endpoint)
		}
	})

	t.Run("path does not begin with slash", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "Test Endpoint",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest" + postfix,
				TypeScriptName: "genTest" + postfix,
			}
		}

		path := strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.Panics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.Panics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.Panics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.Panics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.Panics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest" + postfix,
				TypeScriptName: "genTest" + postfix,
			}
		}

		path := "/" + strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.Panics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.Panics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.Panics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.Panics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.Panics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")
	})

	t.Run("endpoint duplicate path method", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "Test Endpoint",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest" + postfix,
				TypeScriptName: "genTest" + postfix,
			}
		}

		path := "/" + strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.NotPanics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.NotPanics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.NotPanics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.NotPanics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.NotPanics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")

		assert.Panics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.Panics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.Panics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.Panics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.Panics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")
	})

	t.Run("endpoint duplicate GoName", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "Test Endpoint",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest",
				TypeScriptName: "genTest" + postfix,
			}
		}

		path := "/" + strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.NotPanics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.Panics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.Panics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.Panics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.Panics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")
	})

	t.Run("endpoint duplicate TypeScriptName", func(t *testing.T) {
		endpointFn := func(postfix string) *Endpoint {
			return &Endpoint{
				Name:           "Test Endpoint",
				Description:    "This is an Endpoint purely for testing purposes",
				GoName:         "GenTest" + postfix,
				TypeScriptName: "genTest",
			}
		}

		path := "/" + strconv.Itoa(rand.Int())
		eg := EndpointGroup{}

		assert.NotPanics(t, func() { eg.Patch(path, endpointFn(http.MethodPatch)) }, "Patch")
		assert.Panics(t, func() { eg.Get(path, endpointFn(http.MethodGet)) }, "Get")
		assert.Panics(t, func() { eg.Post(path, endpointFn(http.MethodPost)) }, "Post")
		assert.Panics(t, func() { eg.Put(path, endpointFn(http.MethodPut)) }, "Put")
		assert.Panics(t, func() { eg.Delete(path, endpointFn(http.MethodDelete)) }, "Delete")
	})
}
