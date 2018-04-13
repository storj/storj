// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPRanger(t *testing.T) {
	var content string
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, "test", time.Now(), strings.NewReader(content))
		}))
	defer ts.Close()

	for _, example := range []struct {
		data                 string
		size, offset, length int64
		substr               string
		fail                 bool
	}{
		{"", 0, 0, 0, "", false},
		{"abcdef", 6, 0, 0, "", false},
		{"abcdef", 6, 0, 6, "abcdef", false},
		{"abcdef", 6, 0, 5, "abcde", false},
		{"abcdef", 6, 0, 4, "abcd", false},
		{"abcdef", 6, 1, 4, "bcde", false},
		{"abcdef", 6, 2, 4, "cdef", false},
		{"abcdefg", 7, 1, 4, "bcde", false},
		{"abcdef", 6, 0, 7, "abcdef", true},
		{"abcdef", 6, -1, 7, "abcde", true},
	} {
		content = example.data
		r, err := HTTPRanger(ts.URL)
		if r.Size() != example.size {
			t.Fatalf("invalid size: %v != %v", r.Size(), example.size)
		}
		data, err := ioutil.ReadAll(r.Range(example.offset, example.length))
		if example.fail {
			if err == nil {
				t.Fatalf("expected error")
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !bytes.Equal(data, []byte(example.substr)) {
				t.Fatalf("invalid subrange: %#v != %#v", string(data), example.substr)
			}
		}
	}
}
