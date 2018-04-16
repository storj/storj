// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

const (
	testTitle       = "Storj Bridge"
	testDescription = "Some description"
	testVersion     = "1.2.3"
	testHost        = "1.2.3.4"
)

func TestUnmarshalJSON(t *testing.T) {
	for i, tt := range []struct {
		json      string
		info      Info
		errString string
	}{
		{"", Info{}, "unexpected end of JSON input"},
		{"{", Info{}, "unexpected end of JSON input"},
		{"{}", Info{}, ""},
		{`{"info":{}}`, Info{}, ""},
		{`{"info":10}`, Info{}, ""},
		{`{"info":{"title":10,"description":10,"version":10},"host":10}`, Info{}, ""},
		{fmt.Sprintf(`{"info":{"description":"%s","version":"%s"},"host":"%s"}`,
			testDescription, testVersion, testHost),
			Info{
				Description: testDescription,
				Version:     testVersion,
				Host:        testHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","version":"%s"},"host":"%s"}`,
			testTitle, testVersion, testHost),
			Info{
				Title:   testTitle,
				Version: testVersion,
				Host:    testHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s"},"host":"%s"}`,
			testTitle, testDescription, testHost),
			Info{
				Title:       testTitle,
				Description: testDescription,
				Host:        testHost,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"}}`,
			testTitle, testDescription, testVersion),
			Info{
				Title:       testTitle,
				Description: testDescription,
				Version:     testVersion,
			},
			""},
		{fmt.Sprintf(`{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
			testTitle, testDescription, testVersion, testHost),
			Info{
				Title:       testTitle,
				Description: testDescription,
				Version:     testVersion,
				Host:        testHost,
			},
			""},
	} {
		var info Info
		err := json.Unmarshal([]byte(tt.json), &info)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.info, info, errTag)
		}
	}
}

func TestGetInfo(t *testing.T) {
	router := httprouter.New()
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprintf(w, `{"info":{"title":"%s","description":"%s","version":"%s"},"host":"%s"}`,
			testTitle, testDescription, testVersion, testHost)
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	for i, tt := range []struct {
		env       Env
		errString string
	}{
		{NewTestEnv(ts), ""},
		{Env{URL: ts.URL + "/info"}, "unexpected status code: 404"},
	} {
		info, err := GetInfo(tt.env)
		errTag := fmt.Sprintf("Test case #%d", i)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, testTitle, info.Title, errTag)
			assert.Equal(t, testDescription, info.Description, errTag)
			assert.Equal(t, testVersion, info.Version, errTag)
			assert.Equal(t, testHost, info.Host, errTag)
		}
	}
}
