// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package httpmock

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// Response represents a mocked HTTP response.
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

// Transport is a custom HTTP transport for handling mocked responses.
type Transport struct {
	responses map[string][]Response
	mutex     sync.RWMutex
}

// NewTransport creates a new instance of Transport.
func NewTransport() *Transport {
	return &Transport{
		responses: make(map[string][]Response),
	}
}

// AddResponse registers a response for a given URL.
// Multiple responses for the same URL will be returned in sequence.
func (t *Transport) AddResponse(url string, response Response) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.responses[url] = append(t.responses[url], response)
}

// RoundTrip implements the http.RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if responses, ok := t.responses[req.URL.String()]; ok && len(responses) > 0 {
		response := responses[0]
		// Remove the first response after using it
		t.responses[req.URL.String()] = responses[1:]

		headers := make(http.Header)
		for key, value := range response.Headers {
			headers.Set(key, value)
		}

		return &http.Response{
			StatusCode: response.StatusCode,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(response.Body)),
			Request:    req,
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Request:    req,
	}, nil
}

// NewClient creates an *http.Client configured to use the Transport.
func NewClient() (*http.Client, *Transport) {
	transport := NewTransport()
	client := &http.Client{Transport: transport}
	return client, transport
}
