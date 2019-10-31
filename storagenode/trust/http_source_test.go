// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSourceNew(t *testing.T) {
	for _, tt := range []struct {
		name    string
		httpURL string
		err     string
	}{
		{
			name:    "not a valid URL",
			httpURL: "://",
			err:     `trust: invalid HTTP source "://": not a URL: parse ://: missing protocol scheme`,
		},
		{
			name:    "not an HTTP or HTTPS URL",
			httpURL: "file://",
			err:     `trust: invalid HTTP source "file://": scheme is not supported`,
		},
		{
			name:    "missing host",
			httpURL: "http:///path",
			err:     `trust: invalid HTTP source "http:///path": host is missing`,
		},
		{
			name:    "fragment not allowed",
			httpURL: "http://localhost/path#OHNO",
			err:     `trust: invalid HTTP source "http://localhost/path#OHNO": fragment is not allowed`,
		},
		{
			name:    "success",
			httpURL: "http://localhost/path",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHTTPSource(tt.httpURL)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHTTPSourceString(t *testing.T) {
	source, err := NewHTTPSource("http://localhost:1234/path")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:1234/path", source.String())
}

func TestHTTPSourceIsNotFixed(t *testing.T) {
	source, err := NewHTTPSource("http://localhost/path")
	require.NoError(t, err)
	require.False(t, source.Fixed(), "HTTP source is unexpectedly fixed")
}

func TestHTTPSourceFetchEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != "GET":
			http.Error(w, fmt.Sprintf("%s method not allowed", r.Method), http.StatusMethodNotAllowed)
		case r.URL.Path == "/good":
			fmt.Fprintln(w, `
				# Some comment
				121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777
				12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777
			`)
		case r.URL.Path == "/bad":
			fmt.Fprintln(w, "BAD")
		case r.URL.Path == "/ugly":
			http.Error(w, "OHNO", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	url1, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@127.0.0.1:7777")
	require.NoError(t, err)

	url2, err := ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777")
	require.NoError(t, err)

	goodURL := server.URL + "/good"
	badURL := server.URL + "/bad"
	uglyURL := server.URL + "/ugly"

	for _, tt := range []struct {
		name    string
		httpURL string
		err     string
		entries []Entry
	}{
		{
			name:    "well-formed list was fetched",
			httpURL: goodURL,
			entries: []Entry{
				{
					SatelliteURL:  url1,
					Authoritative: true,
				},
				{
					SatelliteURL:  url2,
					Authoritative: false,
				},
			},
		},
		{
			name:    "malformed list was fetched",
			httpURL: badURL,
			err:     "trust: invalid satellite URL: must contain an ID",
		},
		{
			name:    "endpoint returned unsuccessful status code",
			httpURL: uglyURL,
			err:     `trust: unexpected status code 500: "OHNO"`,
		},
		{
			name:    "endpoint returned unsuccessful status code",
			httpURL: uglyURL,
			err:     `trust: unexpected status code 500: "OHNO"`,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewHTTPSource(tt.httpURL)
			require.NoError(t, err)
			entries, err := source.FetchEntries(context.Background())
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.entries, entries)
		})
	}
}

func TestURLMatchesHTTPSourceHost(t *testing.T) {
	for _, tt := range []struct {
		name       string
		urlHost    string
		sourceHost string
		matches    bool
	}{
		{
			name:       "URL IP and source domain should not match",
			urlHost:    "1.2.3.4",
			sourceHost: "domain.test",
			matches:    false,
		},
		{
			name:       "URL domain and source IP should not match",
			urlHost:    "domain.test",
			sourceHost: "1.2.3.4",
			matches:    false,
		},
		{
			name:       "equal URL and source IP should match",
			urlHost:    "1.2.3.4",
			sourceHost: "1.2.3.4",
			matches:    true,
		},
		{
			name:       "inequal URL and source IP should not match",
			urlHost:    "1.2.3.4",
			sourceHost: "4.3.2.1",
			matches:    false,
		},
		{
			name:       "equal URL and source domains should match",
			urlHost:    "domain.test",
			sourceHost: "domain.test",
			matches:    true,
		},
		{
			name:       "URL domain and source subdomains should not match",
			urlHost:    "domain.test",
			sourceHost: "sub.domain.test",
			matches:    false,
		},
		{
			name:       "URL subdomain and source domain should match",
			urlHost:    "sub.domain.test",
			sourceHost: "domain.test",
			matches:    true,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.matches, URLMatchesHTTPSourceHost(tt.urlHost, tt.sourceHost))
		})
	}
}
