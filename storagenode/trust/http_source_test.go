// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/storagenode/trust"
)

func TestHTTPSourceNew(t *testing.T) {
	for _, tt := range []struct {
		name    string
		httpURL string
		errs    []string
	}{
		{
			name:    "not a valid URL",
			httpURL: "://",
			errs: []string{
				`HTTP source: "://": not a URL: parse ://: missing protocol scheme`,
				`HTTP source: "://": not a URL: parse "://": missing protocol scheme`,
			},
		},
		{
			name:    "not an HTTP or HTTPS URL",
			httpURL: "file://",
			errs:    []string{`HTTP source: "file://": scheme is not supported`},
		},
		{
			name:    "missing host",
			httpURL: "http:///path",
			errs:    []string{`HTTP source: "http:///path": host is missing`},
		},
		{
			name:    "fragment not allowed",
			httpURL: "http://localhost/path#OHNO",
			errs:    []string{`HTTP source: "http://localhost/path#OHNO": fragment is not allowed`},
		},
		{
			name:    "success",
			httpURL: "http://localhost/path",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			_, err := trust.NewHTTPSource(tt.httpURL)
			if len(tt.errs) > 0 {
				require.Error(t, err)
				require.Contains(t, tt.errs, err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHTTPSourceString(t *testing.T) {
	source, err := trust.NewHTTPSource("http://localhost:1234/path")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:1234/path", source.String())
}

func TestHTTPSourceIsNotStatic(t *testing.T) {
	source, err := trust.NewHTTPSource("http://localhost/path")
	require.NoError(t, err)
	require.False(t, source.Static(), "HTTP source is unexpectedly static")
}

func TestHTTPSourceFetchEntries(t *testing.T) {
	url1 := makeSatelliteURL("127.0.0.1")
	url2 := makeSatelliteURL("domain.test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != http.MethodGet:
			http.Error(w, r.Method+" method not allowed", http.StatusMethodNotAllowed)
		case r.URL.Path == "/good":
			_, _ = fmt.Fprintf(w, `
				# Some comment
				%s
				%s
			`, url1.String(), url2.String())
		case r.URL.Path == "/bad":
			_, _ = fmt.Fprintln(w, "BAD")
		case r.URL.Path == "/ugly":
			http.Error(w, "OHNO", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	goodURL := server.URL + "/good"
	badURL := server.URL + "/bad"
	uglyURL := server.URL + "/ugly"

	for _, tt := range []struct {
		name    string
		httpURL string
		err     string
		entries []trust.Entry
	}{
		{
			name:    "well-formed list was fetched",
			httpURL: goodURL,
			entries: []trust.Entry{
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
			err:     fmt.Sprintf("HTTP source: cannot parse list at %q: invalid satellite URL: must contain an ID", badURL),
		},
		{
			name:    "endpoint returned unsuccessful status code",
			httpURL: uglyURL,
			err:     fmt.Sprintf(`HTTP source: %q: unexpected status code 500: "OHNO"`, uglyURL),
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := trust.NewHTTPSource(tt.httpURL)
			require.NoError(t, err)
			entries, err := source.FetchEntries(t.Context())
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
			require.Equal(t, tt.matches, trust.URLMatchesHTTPSourceHost(tt.urlHost, tt.sourceHost))
		})
	}
}
