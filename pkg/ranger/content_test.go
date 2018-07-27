// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"net/http/httptest"
)

func TestServeContent(t *testing.T) {
	type RangerArgs struct {
		a string
	}
	for _, tt := range []struct {
		testName         string
		requestMethod    string
		requestHeaderMap map[string]string
		writerHeaderMap  map[string]string
		name             string
		modtime          time.Time
		content          Ranger
	}{
		{
			name:             "True preconditions",
			requestHeaderMap: map[string]string{"If-Match": "\t\t"},
		},
	} {
		t.Run(tt.testName, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, "/", nil)
			for k, v := range tt.requestHeaderMap {
				req.Header.Add(k, v)
			}

			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			ServeContent(context.Background(), writer, req, tt.name, tt.modtime, tt.content)
		})
	}
}

func TestServeContentContentSize(t *testing.T) {
	req := httptest.NewRequest("", "/", nil)
	writer := httptest.NewRecorder()
	ranger := ByteRanger([]byte(""))

	ServeContent(context.Background(), writer, req, "", time.Now().UTC(), ranger)

	assert.Equal(t, http.StatusOK, writer.Code)
}

func TestServeContentParseRange(t *testing.T) {
	req := httptest.NewRequest("", "/", nil)
	for k, v := range map[string]string{"If-Range": "\"abcde\""} {
		req.Header.Add(k, v)
	}

	writer := httptest.NewRecorder()
	for k, v := range map[string]string{"Etag": "\"abcde\""} {
		writer.Header().Add(k, v)
	}
	ranger := ByteRanger([]byte("bytes=1-5/0,bytes=1-5/8",))

	ServeContent(context.Background(), writer, req, "", time.Now().UTC(), ranger)

	assert.Equal(t, http.StatusOK, writer.Code)
	assert.Equal(t, "23", writer.HeaderMap.Get("Content-Length"))
}

func Test_isZeroTime(t *testing.T) {
	for _, tt := range []struct {
		name     string
		t        time.Time
		expected bool
	}{
		{
			name:     "Valid",
			t:        time.Now().UTC(),
			expected: false,
		},

		{
			name:     "Zero time",
			t:        time.Unix(0, 0).UTC(),
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := isZeroTime(tt.t)

			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_setLastModified(t *testing.T) {
	for _, tt := range []struct {
		name     string
		modtime  time.Time
		expected string
	}{
		{
			name:     "Zero time",
			modtime:  time.Unix(0, 0).UTC(),
			expected: "",
		},
		{
			name:     "Valid time",
			modtime:  time.Unix(1531836358, 0).UTC(),
			expected: "Tue, 17 Jul 2018 14:05:58 GMT",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRecorder()

			setLastModified(req, tt.modtime)

			assert.Equal(t, tt.expected, req.HeaderMap.Get("Last-Modified"))
		})
	}
}

//TODO This code block is commented until PR #158 acceptance
//func Test_setLastModifiedNilWriter(t *testing.T) {
//	req := httptest.NewRecorder()
//
//	setLastModified(nil, time.Now().UTC())
//
//	assert.Equal(t, "", req.HeaderMap.Get("Last-Modified"))
//}

func Test_checkPreconditions(t *testing.T) {
	for _, tt := range []struct {
		name                string
		requestHeaderMap    map[string]string
		writerHeaderMap     map[string]string
		requestMethod       string
		modtime             time.Time
		expectedDone        bool
		expectedRangeHeader string
	}{
		{
			name:                "Empty If-Match with trailing spaces",
			requestHeaderMap:    map[string]string{"If-Match": "\t\t"},
			expectedDone:        true,
			expectedRangeHeader: "",
		},

		{
			name:                "No If-Match header",
			requestHeaderMap:    map[string]string{"If-Unmodified-Since": "Thursday, 18-Jul-18 12:20:25 EEST"},
			expectedDone:        true,
			modtime:             time.Unix(1531999477, 0).UTC(),
			expectedRangeHeader: "",
		},

		{
			name:                "Any If-Match header with GET request",
			requestMethod:       http.MethodGet, //By default method is GET. Wrote for clarity
			requestHeaderMap:    map[string]string{"If-None-Match": "*"},
			expectedDone:        true,
			expectedRangeHeader: "",
		},

		{
			name:                "Any If-Match header with HEAD request",
			requestHeaderMap:    map[string]string{"If-None-Match": "*"},
			requestMethod:       http.MethodHead,
			expectedDone:        true,
			expectedRangeHeader: "",
		},

		{
			name:                "Any If-Match header with PUT request",
			requestHeaderMap:    map[string]string{"If-None-Match": "*"},
			requestMethod:       http.MethodPut,
			expectedDone:        true,
			expectedRangeHeader: "",
		},

		{
			name:                "Empty request",
			requestHeaderMap:    map[string]string{},
			expectedDone:        false,
			expectedRangeHeader: "",
		},

		{
			name:                "Empty modified request",
			requestHeaderMap:    map[string]string{"If-Modified-Since": "Thursday, 20-Jul-18 12:20:25 EEST"},
			modtime:             time.Unix(1531999477, 0).UTC(),
			expectedDone:        true,
			expectedRangeHeader: "",
		},

		{
			name:                "Empty unmodified request",
			requestHeaderMap:    map[string]string{"Range": "aaa", "If-Range": "\"abcde\""},
			expectedDone:        false,
			expectedRangeHeader: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, "/", nil)
			for k, v := range tt.requestHeaderMap {
				req.Header.Add(k, v)
			}

			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			gotDone, gotRangeHeader := checkPreconditions(writer, req, tt.modtime)

			assert.Equal(t, tt.expectedDone, gotDone)
			assert.Equal(t, tt.expectedRangeHeader, gotRangeHeader)
		})
	}
}

func Test_checkIfMatch(t *testing.T) {
	for _, tt := range []struct {
		name             string
		requestHeaderMap map[string]string
		writerHeaderMap  map[string]string
		expectedResult   condResult
	}{
		{
			name:             "No If-Match header",
			requestHeaderMap: map[string]string{},
			expectedResult:   condNone,
		},

		{
			name:             "Empty If-Match with trailing spaces",
			requestHeaderMap: map[string]string{"If-Match": "\t\t"},
			expectedResult:   condFalse,
		},

		{
			name:             "Starts with coma",
			requestHeaderMap: map[string]string{"If-Match": ","},
			expectedResult:   condFalse,
		},

		{
			name:             "Anything match",
			requestHeaderMap: map[string]string{"If-Match": "*"},
			expectedResult:   condTrue,
		},

		{
			name:             "Empty If-Match ETag",
			requestHeaderMap: map[string]string{"If-Match": "abcd"},
			expectedResult:   condFalse,
		},

		{
			name:             "First ETag match",
			requestHeaderMap: map[string]string{"If-Match": "\"testETag\", \"testETag\""},
			writerHeaderMap:  map[string]string{"Etag": "\"testETag\""},
			expectedResult:   condTrue,
		},

		{
			name:             "Separated ETag match",
			requestHeaderMap: map[string]string{"If-Match": "\"wrongETag\", \"correctETag\""},
			writerHeaderMap:  map[string]string{"Etag": "\"correctETag\""},
			expectedResult:   condTrue,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("", "/", nil)
			for k, v := range tt.requestHeaderMap {
				req.Header.Add(k, v)
			}

			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			gotResult := checkIfMatch(writer, req)

			assert.Equal(t, tt.expectedResult, gotResult)
		})
	}
}

func Test_checkIfUnmodifiedSince(t *testing.T) {
	for _, tt := range []struct {
		name           string
		headerMap      map[string]string
		modtime        time.Time
		expectedResult condResult
	}{
		{
			name:           "No modified flag",
			headerMap:      map[string]string{},
			modtime:        time.Now().UTC(),
			expectedResult: condNone,
		},

		{
			name:           "Zero time",
			headerMap:      map[string]string{"If-Unmodified-Since": "Thursday, 18-Jul-18 12:20:25 EEST"},
			modtime:        time.Unix(0, 0).UTC(),
			expectedResult: condNone,
		},

		{
			name:           "Is modified",
			headerMap:      map[string]string{"If-Unmodified-Since": "Thursday, 20-Jul-18 12:20:25 EEST"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condTrue,
		},

		{
			name:           "Is not modified",
			headerMap:      map[string]string{"If-Unmodified-Since": "Thursday, 18-Jul-18 12:20:25 EEST"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condFalse,
		},

		{
			name:           "Malformed RFC time",
			headerMap:      map[string]string{"If-Unmodified-Since": "abcdefg"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condNone,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("", "/", nil)
			for k, v := range tt.headerMap {
				req.Header.Add(k, v)
			}

			gotResult := checkIfUnmodifiedSince(req, tt.modtime)

			assert.Equal(t, tt.expectedResult, gotResult)
		})
	}
}

func Test_checkIfNoneMatch(t *testing.T) {
	for _, tt := range []struct {
		name             string
		requestHeaderMap map[string]string
		writerHeaderMap  map[string]string
		expectedResult   condResult
	}{
		{
			name:             "No If-None-Match header",
			requestHeaderMap: map[string]string{},
			expectedResult:   condNone,
		},

		{
			name:             "Empty If-None-Match with trailing spaces",
			requestHeaderMap: map[string]string{"If-None-Match": "\t\t"},
			expectedResult:   condTrue,
		},

		{
			name:             "Starts with coma",
			requestHeaderMap: map[string]string{"If-None-Match": ","},
			expectedResult:   condTrue,
		},

		{
			name:             "Anything match",
			requestHeaderMap: map[string]string{"If-None-Match": "*"},
			expectedResult:   condFalse,
		},

		{
			name:             "Empty If-None-Match ETag",
			requestHeaderMap: map[string]string{"If-None-Match": "abcd"},
			expectedResult:   condTrue,
		},

		{
			name:             "First ETag match",
			requestHeaderMap: map[string]string{"If-None-Match": "\"testETag\", \"testETag\""},
			writerHeaderMap:  map[string]string{"Etag": "\"testETag\""},
			expectedResult:   condFalse,
		},

		{
			name:             "Separated ETag match",
			requestHeaderMap: map[string]string{"If-None-Match": "\"wrongETag\", \"correctETag\""},
			writerHeaderMap:  map[string]string{"Etag": "\"correctETag\""},
			expectedResult:   condFalse,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("", "/", nil)
			for k, v := range tt.requestHeaderMap {
				req.Header.Add(k, v)
			}

			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			gotResult := checkIfNoneMatch(writer, req)

			assert.Equal(t, tt.expectedResult, gotResult)
		})
	}
}

func Test_checkIfModifiedSince(t *testing.T) {
	for _, tt := range []struct {
		name           string
		requestMethod  string
		headerMap      map[string]string
		modtime        time.Time
		expectedResult condResult
	}{
		{
			name:           "Unacceptable Method",
			requestMethod:  "PUT",
			headerMap:      map[string]string{},
			expectedResult: condNone,
		},

		{
			name:           "No If-Modified-Since header",
			requestMethod:  "GET",
			expectedResult: condNone,
		},

		{
			name:           "Malformed If-Modified-Since header",
			requestMethod:  "GET",
			headerMap:      map[string]string{"If-Modified-Since": "aaaa"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condNone,
		},

		{
			name:           "Is not modified before",
			requestMethod:  "GET",
			headerMap:      map[string]string{"If-Modified-Since": "Thursday, 20-Jul-18 12:20:25 EEST"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condFalse,
		},

		{
			name:           "Modified before",
			requestMethod:  "GET",
			headerMap:      map[string]string{"If-Modified-Since": "Thursday, 18-Jul-18 12:20:25 EEST"},
			modtime:        time.Unix(1531999477, 0).UTC(),
			expectedResult: condTrue,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, "/", nil)
			for k, v := range tt.headerMap {
				req.Header.Add(k, v)
			}

			gotResult := checkIfModifiedSince(req, tt.modtime)

			assert.Equal(t, tt.expectedResult, gotResult)
		})
	}
}

func Test_checkIfRange(t *testing.T) {
	for _, tt := range []struct {
		name             string
		requestMethod    string
		requestHeaderMap map[string]string
		writerHeaderMap  map[string]string
		modtime          time.Time
		expectedResult   condResult
	}{
		{
			name:           "Unacceptable Method",
			requestMethod:  "PUT",
			expectedResult: condNone,
		},

		{
			name:           "No If-Range header",
			requestMethod:  "GET",
			expectedResult: condNone,
		},

		{
			name:             "Not matching ETags",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "\"abcde\""},
			expectedResult:   condFalse,
		},

		{
			name:             "Matching ETags",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "\"abcde\""},
			writerHeaderMap:  map[string]string{"Etag": "\"abcde\""},
			expectedResult:   condTrue,
		},

		//TODO This test case increases test coverage of 'checkIfRange' function to 100%
		//Waiting PR #158 to be accepted
		{
			name:             "Zero modtime",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "a"},
			modtime:          time.Unix(0, 0).UTC(),
			expectedResult:   condFalse,
		},

		{
			name:             "Malformed header time",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "aaa"},
			modtime:          time.Unix(1531999477, 0).UTC(),
			expectedResult:   condFalse,
		},

		{
			name:             "Equal time",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "Thu, 19 Jul 2018 14:12:03 GMT"},
			modtime:          time.Unix(1532009523, 0).UTC(),
			expectedResult:   condTrue,
		},

		{
			name:             "Equal time",
			requestMethod:    "GET",
			requestHeaderMap: map[string]string{"If-Range": "Thu, 18 Jul 2018 14:12:03 GMT"},
			modtime:          time.Unix(1532009523, 0).UTC(),
			expectedResult:   condFalse,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, "/", nil)
			for k, v := range tt.requestHeaderMap {
				req.Header.Add(k, v)
			}

			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			gotResult := checkIfRange(writer, req, tt.modtime)

			assert.Equal(t, tt.expectedResult, gotResult)
		})
	}
}

func Test_writeNotModified(t *testing.T) {

	for _, tt := range []struct {
		name            string
		writerHeaderMap map[string]string
	}{
		{
			name: "Empty ETag",
			writerHeaderMap: map[string]string{
				"Content-Type":   "a",
				"Content-Length": "23",
				"Etag":           "",
			},
		},

		{
			name: "Existing ETag",
			writerHeaderMap: map[string]string{
				"Content-Type":   "a",
				"Content-Length": "23",
				"Etag":           "asdf",
				"Last-Modified":  "aaa",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			for k, v := range tt.writerHeaderMap {
				writer.Header().Add(k, v)
			}

			writeNotModified(writer)

			assert.Equal(t, http.StatusNotModified, writer.Code)
			assert.Equal(t, "", writer.Header().Get("Content-Type"))
			assert.Equal(t, "", writer.Header().Get("Content-Length"))
			assert.Equal(t, "", writer.Header().Get("Last-Modified"))
		})
	}
}

//TODO Waiting PR #158 to be accepted for 100% coverage
func Test_scanETag(t *testing.T) {
	for _, tt := range []struct {
		name           string
		s              string
		expectedEtag   string
		expectedRemain string
	}{
		{
			name:           "Empty ETag", s: "",
			expectedEtag:   "",
			expectedRemain: "",
		},

		{
			name:           "Empty ETag with W", s: "W/",
			expectedEtag:   "",
			expectedRemain: "",
		},

		{
			name:           "Malformed ETag",
			s:              "asdf",
			expectedEtag:   "",
			expectedRemain: "",
		},

		{
			name:           "Valid ETag",
			s:              "\"abcdef\"",
			expectedEtag:   "\"abcdef\"",
			expectedRemain: "",
		},

		{
			name:           "Valid ETag with W/",
			s:              "W/\"aaaa\"",
			expectedEtag:   "W/\"aaaa\"",
			expectedRemain: "",
		},

		//{
		//	name:           "Valid ETag with special character",
		//	s:              "\"{aa\"",
		//	expectedEtag:   "",
		//	expectedRemain: "",
		//},
	} {
		t.Run(tt.name, func(t *testing.T) {

			gotEtag, gotRemain := scanETag(tt.s)

			assert.Equal(t, tt.expectedEtag, gotEtag)
			assert.Equal(t, tt.expectedRemain, gotRemain)
		})
	}
}

func Test_etagStrongMatch(t *testing.T) {
	for _, tt := range []struct {
		name          string
		a, b          string
		expectedMatch bool
	}{
		{
			name:          "Not equal arguments",
			a:             "a",
			b:             "b",
			expectedMatch: false,
		},
		{
			name:          "Empty string",
			a:             "",
			b:             "",
			expectedMatch: false,
		},
		{
			name:          "Does not start with required char",
			a:             "a",
			b:             "a",
			expectedMatch: false,
		},
		{
			name:          "Valid test case",
			a:             "\"",
			b:             "\"",
			expectedMatch: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch := etagStrongMatch(tt.a, tt.b)

			assert.Equal(t, tt.expectedMatch, gotMatch)
		})
	}
}

func Test_etagWeakMatch(t *testing.T) {
	for _, tt := range []struct {
		name          string
		a, b          string
		expectedMatch bool
	}{
		{
			name:          "Empty ETag",
			a:             "",
			b:             "",
			expectedMatch: true,
		},

		{
			name:          "Not equal ETags",
			a:             "W/a",
			b:             "W/b",
			expectedMatch: false,
		},

		{
			name:          "Equal ETags",
			a:             "W/a",
			b:             "W/a",
			expectedMatch: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch := etagWeakMatch(tt.a, tt.b)

			assert.Equal(t, tt.expectedMatch, gotMatch)
		})
	}
}

func Test_httpRange_contentRange(t *testing.T) {
	for _, tt := range []struct {
		name              string
		start             int64
		length            int64
		size              int64
		expectedRangeSize string
	}{
		{
			name:              "Valid case",
			start:             1,
			length:            5,
			size:              8,
			expectedRangeSize: "bytes 1-5/8",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httpRange{
				start:  tt.start,
				length: tt.length,
			}

			gotRangeSize := r.contentRange(tt.size)

			assert.Equal(t, tt.expectedRangeSize, gotRangeSize)
		})
	}
}

func Test_httpRange_mimeHeader(t *testing.T) {
	for _, tt := range []struct {
		name        string
		contentType string
		size        int64
		expected    string
	}{
		{
			name:        "Valid",
			contentType: "text",
			size:        8,
			expected:    "bytes 1-5/8",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httpRange{
				start:  1,
				length: 5,
			}

			gotMimeHeader := r.mimeHeader(tt.contentType, tt.size)

			assert.Equal(t, tt.contentType, gotMimeHeader.Get("Content-Type"))
			assert.Equal(t, tt.expected, gotMimeHeader.Get("Content-Range"))
		})
	}
}

func Test_parseRange(t *testing.T) {
	for _, tt := range []struct {
		name          string
		s             string
		size          int64
		expectedRange []httpRange
		expectedError bool
	}{
		{
			name:          "Header not present",
			s:             "",
			size:          0,
			expectedRange: nil,
			expectedError: false,
		},
		{
			name:          "invalid range",
			s:             "a",
			size:          0,
			expectedRange: nil,
			expectedError: true,
		},
		{
			name:          "Empty Bytes",
			s:             "bytes=",
			size:          0,
			expectedRange: nil,
			expectedError: false,
		},
		{
			name:          "invalid Range",
			s:             "bytes=1-5/0,bytes=1-5/8",
			size:          0,
			expectedRange: nil,
			expectedError: true,
		},
		{
			name:          "invalid Range",
			s:             "bytes=-",
			size:          0,
			expectedRange: nil,
			expectedError: true,
		},
		{
			name:          "invalid Range",
			s:             "bytes=111,bytes=111,bytes=111",
			size:          0,
			expectedRange: nil,
			expectedError: true,
		},
		{
			name:          "invalid Range",
			s:             "bytes=1-5/0,bytes=1-5/8",
			size:          3,
			expectedRange: nil,
			expectedError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotRange, err := parseRange(tt.s, tt.size)

			assert.Equal(t, err != nil, tt.expectedError)
			assert.Equal(t, gotRange, tt.expectedRange)
		})

	}
}

func Test_countingWriter_Write(t *testing.T) {
	for _, tt := range []struct {
		name           string
		arrayHolder    string
		expectedLength int
		expectingError bool
	}{
		{
			name:           "",
			arrayHolder:    "abcd",
			expectedLength: 4,
			expectingError: false,
		},

		{
			name:           "",
			arrayHolder:    "",
			expectedLength: 0,
			expectingError: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var a countingWriter

			gotN, err := a.Write([]byte(tt.arrayHolder))

			assert.Equal(t, tt.expectedLength, gotN)
			assert.Equal(t, tt.expectingError, err != nil)
		})
	}
}

func Test_rangesMIMESize(t *testing.T) {
	for _, tt := range []struct {
		name            string
		ranges          []httpRange
		expectedEncSize int64
	}{
		{
			name: "Valid case",
			ranges: []httpRange{
				{start: 0, length: 5},
			},
			expectedEncSize: 187,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotEncSize := rangesMIMESize(tt.ranges, "text", 3)

			assert.Equal(t, tt.expectedEncSize, gotEncSize)
		})
	}
}

func Test_sumRangesSize(t *testing.T) {
	for _, tt := range []struct {
		ranges       []httpRange
		expectedSize int64
	}{
		{
			ranges: []httpRange{
				{length: 5},
				{length: 5},
				{length: 5},
			},
			expectedSize: 15,
		},

		{
			ranges: []httpRange{
				{length: 0},
				{length: 0},
				{length: 0},
			},
			expectedSize: 0,
		},
	} {
		gotSize := sumRangesSize(tt.ranges)

		assert.Equal(t, tt.expectedSize, gotSize)
	}
}
