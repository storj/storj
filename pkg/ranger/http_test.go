// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPRanger(t *testing.T) {
	var content string
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "test", time.Now(), strings.NewReader(content))
		}))
	defer ts.Close()

	for i, tt := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		errString            string
	}{
		{"", 0, 0, 0, "", ""},
		{"abcdef", 6, 0, 0, "", ""},
		{"abcdef", 6, 3, 0, "", ""},
		{"abcdef", 6, 0, 6, "abcdef", ""},
		{"abcdef", 6, 0, 5, "abcde", ""},
		{"abcdef", 6, 0, 4, "abcd", ""},
		{"abcdef", 6, 1, 4, "bcde", ""},
		{"abcdef", 6, 2, 4, "cdef", ""},
		{"abcdefg", 7, 1, 4, "bcde", ""},
		{"abcdef", 6, 0, 7, "abcdef", "ranger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "ranger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "ranger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		content = tt.data
		rr, err := HTTPRanger(ts.URL)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, tt.size, rr.Size(), errTag)
		}
		r, err := rr.Range(context.Background(), tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}

func TestHTTPRangerURLError(t *testing.T) {
	rr, err := HTTPRanger("")
	assert.Nil(t, rr)
	assert.NotNil(t, err)
}

func TestHTTPRangeStatusCodeOk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			http.ServeContent(w, r, "test", time.Now(), strings.NewReader(""))
		}))
	rr, err := HTTPRanger(ts.URL)
	assert.Nil(t, rr)
	assert.NotNil(t, err)
}

func TestHTTPRangerSize(t *testing.T) {
	var content string
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "test", time.Now(), strings.NewReader(content))
		}))
	defer ts.Close()

	for i, tt := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		errString            string
	}{
		{"", 0, 0, 0, "", ""},
		{"abcdef", 6, 0, 0, "", ""},
		{"abcdef", 6, 3, 0, "", ""},
		{"abcdef", 6, 0, 6, "abcdef", ""},
		{"abcdef", 6, 0, 5, "abcde", ""},
		{"abcdef", 6, 0, 4, "abcd", ""},
		{"abcdef", 6, 1, 4, "bcde", ""},
		{"abcdef", 6, 2, 4, "cdef", ""},
		{"abcdefg", 7, 1, 4, "bcde", ""},
		{"abcdef", 6, 0, 7, "abcdef", "ranger error: range beyond end"},
		{"abcdef", 6, -1, 7, "abcde", "ranger error: negative offset"},
		{"abcdef", 6, 0, -1, "abcde", "ranger error: negative length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		content = tt.data
		rr := HTTPRangerSize(ts.URL, tt.size)
		assert.Equal(t, tt.size, rr.Size(), errTag)
		r, err := rr.Range(context.Background(), tt.offset, tt.length)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		assert.NoError(t, err, errTag)
		data, err := ioutil.ReadAll(r)
		if assert.NoError(t, err, errTag) {
			assert.Equal(t, []byte(tt.substr), data, errTag)
		}
	}
}
