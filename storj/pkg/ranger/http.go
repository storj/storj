// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

type httpRanger struct {
	URL  string
	size int64
}

// HTTPRanger turns an HTTP URL into a Ranger
func HTTPRanger(ctx context.Context, url string) (_ Ranger, err error) {
	defer mon.Task()(&ctx)(&err)
	resp, err := http.Head(url)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, Error.New("unexpected status code: %d (expected %d)",
			resp.StatusCode, http.StatusOK)
	}
	contentLength := resp.Header.Get("Content-Length")
	size, err := strconv.Atoi(contentLength)
	if err != nil {
		return nil, err
	}
	return &httpRanger{
		URL:  url,
		size: int64(size),
	}, nil
}

// HTTPRangerSize creates an HTTPRanger with known size.
// Use it if you know the content size. This will safe the extra HEAD request
// for retrieving the content size.
func HTTPRangerSize(url string, size int64) Ranger {
	return &httpRanger{
		URL:  url,
		size: size,
	}
}

// Size implements Ranger.Size
func (r *httpRanger) Size() int64 {
	return r.size
}

// Range implements Ranger.Range
func (r *httpRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	if offset < 0 {
		return nil, Error.New("negative offset")
	}
	if length < 0 {
		return nil, Error.New("negative length")
	}
	if offset+length > r.size {
		return nil, Error.New("range beyond end")
	}
	if length == 0 {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		if err := resp.Body.Close(); err != nil {
			return nil, Error.New("Error: (%v) Failed to close Body :: unexpected status code: %d (expected %d)",
				err, resp.StatusCode, http.StatusPartialContent)
		}

		return nil, Error.New("unexpected status code: %d (expected %d)",
			resp.StatusCode, http.StatusPartialContent)
	}
	return resp.Body, nil
}
