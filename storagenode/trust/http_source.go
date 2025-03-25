// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/zeebo/errs"
)

var (
	// ErrHTTPSource is an error class for HTTP source errors.
	ErrHTTPSource = errs.Class("HTTP source")
)

// HTTPSource represents a trust source at a http:// or https:// URL.
type HTTPSource struct {
	url *url.URL
}

// NewHTTPSource constructs a new HTTPSource from a URL. The URL must be
// an http:// or https:// URL. The fragment cannot be set.
func NewHTTPSource(httpURL string) (*HTTPSource, error) {
	u, err := url.Parse(httpURL)
	if err != nil {
		return nil, ErrHTTPSource.New("%q: not a URL: %w", httpURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrHTTPSource.New("%q: scheme is not supported", httpURL)
	}
	if u.Host == "" {
		return nil, ErrHTTPSource.New(`%q: host is missing`, httpURL)
	}
	if u.Fragment != "" {
		return nil, ErrHTTPSource.New("%q: fragment is not allowed", httpURL)
	}
	return &HTTPSource{url: u}, nil
}

// String implements the Source interface and returns the URL.
func (source *HTTPSource) String() string {
	return source.url.String()
}

// Static implements the Source interface. It returns false for this source.
func (source *HTTPSource) Static() bool { return false }

// FetchEntries implements the Source interface and returns entries parsed from
// the list retrieved over HTTP(S). The entries returned are only authoritative
// if the entry URL has a host that matches or is a subdomain of the source URL.
func (source *HTTPSource) FetchEntries(ctx context.Context) (_ []Entry, err error) {
	defer mon.Task()(&ctx)(&err)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url.String(), nil)
	if err != nil {
		return nil, ErrHTTPSource.Wrap(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ErrHTTPSource.Wrap(err)
	}
	defer func() {
		// Errors closing the response body can be ignored since they don't
		// impact the correctness of the function.
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrHTTPSource.New("%q: unexpected status code %d: %q", source.url, resp.StatusCode, tryReadLine(resp.Body))
	}

	urls, err := ParseSatelliteURLList(ctx, resp.Body)
	if err != nil {
		return nil, ErrHTTPSource.New("cannot parse list at %q: %w", source.url, err)
	}

	var entries []Entry
	for _, url := range urls {
		authoritative := URLMatchesHTTPSourceHost(url.Host, source.url.Hostname())

		entries = append(entries, Entry{
			SatelliteURL:  url,
			Authoritative: authoritative,
		})
	}
	return entries, nil
}

// URLMatchesHTTPSourceHost takes the Satellite URL host and the host of the
// HTTPSource URL and determines if the SatelliteURL matches or is in the
// same domain as the HTTPSource URL.
func URLMatchesHTTPSourceHost(urlHost, sourceHost string) bool {
	urlIP := net.ParseIP(urlHost)
	sourceIP := net.ParseIP(sourceHost)

	// If one is an IP and the other isn't, then this isn't a match.
	// TODO: should we resolve the non-IP host and see if it then matches?
	if (urlIP != nil) != (sourceIP != nil) {
		return false
	}

	// Both are IP addresses. Check for equality.
	if urlIP != nil && sourceIP != nil {
		return urlIP.Equal(sourceIP)
	}

	// Both are domain names. Check if the URL host matches or is a subdomain of
	// the source host.
	urlHost = normalizeHost(urlHost)
	sourceHost = normalizeHost(sourceHost)
	if urlHost == sourceHost {
		return true
	}
	return strings.HasSuffix(urlHost, "."+sourceHost)
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.Trim(host, "."))
}

func tryReadLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Text()
}
