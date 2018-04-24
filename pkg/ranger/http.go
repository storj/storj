// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"storj.io/storj/internal/pkg/readcloser"
)

type httpRanger struct {
	URL  string
	size int64
}

// HTTPRanger turns an HTTP URL into a Ranger
func HTTPRanger(URL string) (Ranger, error) {
	resp, err := http.Head(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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
		URL:  URL,
		size: int64(size),
	}, nil
}

// Size implements Ranger.Size
func (r *httpRanger) Size() int64 {
	return r.size
}

// Range implements Ranger.Range
func (r *httpRanger) Range(offset, length int64) io.ReadCloser {
	if offset < 0 {
		return readcloser.FatalReadCloser(Error.New("negative offset"))
	}
	if length < 0 {
		return readcloser.FatalReadCloser(Error.New("negative length"))
	}
	if offset+length > r.size {
		return readcloser.FatalReadCloser(Error.New("range beyond end"))
	}
	if length == 0 {
		return ioutil.NopCloser(bytes.NewReader([]byte{}))
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return readcloser.FatalReadCloser(err)
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	resp, err := client.Do(req)
	if err != nil {
		return readcloser.FatalReadCloser(err)
	}
	if resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return readcloser.FatalReadCloser(
			Error.New("unexpected status code: %d (expected %d)",
				resp.StatusCode, http.StatusPartialContent))
	}
	return resp.Body
}
