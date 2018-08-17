// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	for i, tt := range []struct {
		path     string
		expected Path
	}{
		{"", []string{}},
		{"/", []string{}},
		{"a", []string{"a"}},
		{"/a/", []string{"a"}},
		{"a/b/c/d", []string{"a", "b", "c", "d"}},
		{"///a//b////c/d///", []string{"a", "b", "c", "d"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestNewWithSegments(t *testing.T) {
	for i, tt := range []struct {
		segs     []string
		expected Path
	}{
		{nil, []string{}},
		{[]string{""}, []string{}},
		{[]string{"", ""}, []string{}},
		{[]string{"/"}, []string{}},
		{[]string{"a"}, []string{"a"}},
		{[]string{"/a/"}, []string{"a"}},
		{[]string{"", "a", "", "b", "c", "d", ""}, []string{"a", "b", "c", "d"}},
		{[]string{"a", "b", "c", "d"}, []string{"a", "b", "c", "d"}},
		{[]string{"/a", "b/", "/c/", "d"}, []string{"a", "b", "c", "d"}},
		{[]string{"a/b", "c", "d"}, []string{"a", "b", "c", "d"}},
		{[]string{"a/b", "c/d"}, []string{"a", "b", "c", "d"}},
		{[]string{"//a/b", "c///d//"}, []string{"a", "b", "c", "d"}},
		{[]string{"a/b/c/d"}, []string{"a", "b", "c", "d"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.segs...)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestString(t *testing.T) {
	for i, tt := range []struct {
		path     Path
		expected string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a/b"},
		{[]string{"a", "b", "c", "d"}, "a/b/c/d"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		s := tt.path.String()
		assert.Equal(t, tt.expected, s, errTag)
	}
}

func TestBytes(t *testing.T) {
	for i, tt := range []struct {
		path     Path
		expected []byte
	}{
		{nil, []byte{}},
		{[]string{}, []byte{}},
		{[]string{""}, []byte{}},
		{[]string{"a/b"}, []byte{97, 47, 98}},
		{[]string{"a/b/c"}, []byte{97, 47, 98, 47, 99}},
		{[]string{"a/b/c/d/e/f"}, []byte{97, 47, 98, 47, 99, 47, 100, 47, 101, 47, 102}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		b := tt.path.Bytes()
		assert.Equal(t, tt.expected, b, errTag)
	}
}

func TestPrepend(t *testing.T) {
	for i, tt := range []struct {
		prefix   string
		path     string
		expected Path
	}{
		{"", "", []string{}},
		{"prefix", "", []string{"prefix"}},
		{"", "my/path", []string{"my", "path"}},
		{"prefix", "my/path", []string{"prefix", "my", "path"}},
		{"p1/p2/p3", "my/path", []string{"p1", "p2", "p3", "my", "path"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path).Prepend(tt.prefix)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestPrependWithSegments(t *testing.T) {
	for i, tt := range []struct {
		segs     []string
		path     string
		expected Path
	}{
		{nil, "", []string{}},
		{[]string{""}, "", []string{}},
		{[]string{"prefix"}, "", []string{"prefix"}},
		{[]string{""}, "my/path", []string{"my", "path"}},
		{[]string{"prefix"}, "my/path", []string{"prefix", "my", "path"}},
		{[]string{"p1/p2/p3"}, "my/path", []string{"p1", "p2", "p3", "my", "path"}},
		{[]string{"p1", "p2/p3"}, "my/path", []string{"p1", "p2", "p3", "my", "path"}},
		{[]string{"p1", "p2", "p3"}, "my/path", []string{"p1", "p2", "p3", "my", "path"}},
		{[]string{"", "p1", "", "", "p2", "p3", ""}, "my/path", []string{"p1", "p2", "p3", "my", "path"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path).Prepend(tt.segs...)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestAppend(t *testing.T) {
	for i, tt := range []struct {
		path     string
		suffix   string
		expected Path
	}{
		{"", "", []string{}},
		{"", "suffix", []string{"suffix"}},
		{"my/path", "", []string{"my", "path"}},
		{"my/path", "suffix", []string{"my", "path", "suffix"}},
		{"my/path", "s1/s2/s3", []string{"my", "path", "s1", "s2", "s3"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path).Append(tt.suffix)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestAppendWithSegments(t *testing.T) {
	for i, tt := range []struct {
		path     string
		segs     []string
		expected Path
	}{
		{"", nil, []string{}},
		{"", []string{""}, []string{}},
		{"", []string{"suffix"}, []string{"suffix"}},
		{"my/path", []string{""}, []string{"my", "path"}},
		{"my/path", []string{"suffix"}, []string{"my", "path", "suffix"}},
		{"my/path", []string{"s1/s2/s3"}, []string{"my", "path", "s1", "s2", "s3"}},
		{"my/path", []string{"s1", "s2/s3"}, []string{"my", "path", "s1", "s2", "s3"}},
		{"my/path", []string{"s1", "s2", "s3"}, []string{"my", "path", "s1", "s2", "s3"}},
		{"my/path", []string{"", "s1", "", "", "s2", "s3", ""}, []string{"my", "path", "s1", "s2", "s3"}},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path).Append(tt.segs...)
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestHasPrefix(t *testing.T) {
	for i, tt := range []struct {
		path     string
		prefix   string
		expected bool
	}{
		{"", "", true},
		{"my/path", "", true},
		{"", "prefix", false},
		{"prefix/path", "prefix", true},
		{"prefix/path", "prefix/path", true},
		{"prefix/path", "prefix/path/more", false},
		{"my/path/s1/s2/s3", "my/path", true},
		{"my/path/s1/s2/s3", "s1/s2/s3", false},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		p := New(tt.path).HasPrefix(New(tt.prefix))
		assert.Equal(t, tt.expected, p, errTag)
	}
}

func TestEncryption(t *testing.T) {
	for i, segs := range []Path{
		nil,          // empty path
		[]string{},   // empty path
		[]string{""}, // empty path segment
		[]string{"file.txt"},
		[]string{"fold1", "file.txt"},
		[]string{"fold1", "fold2", "file.txt"},
		[]string{"fold1", "fold2", "fold3", "file.txt"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		path := New(segs...)
		key := []byte("my secret")
		encrypted, err := path.Encrypt(key)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		decrypted, err := encrypted.Decrypt(key)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		assert.Equal(t, path, decrypted, errTag)
	}
}

func TestDeriveKey(t *testing.T) {
	for i, tt := range []struct {
		path      Path
		depth     int
		errString string
	}{
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, -1,
			"paths error: negative depth"},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 0, ""},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 1, ""},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 2, ""},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 3, ""},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 4, ""},
		{[]string{"fold1", "fold2", "fold3", "file.txt"}, 5,
			"paths error: depth greater than path length"},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)
		key := []byte("my secret")
		encrypted, err := tt.path.Encrypt(key)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		derivedKey, err := tt.path.DeriveKey(key, tt.depth)
		if tt.errString != "" {
			assert.EqualError(t, err, tt.errString, errTag)
			continue
		}
		if !assert.NoError(t, err, errTag) {
			continue
		}
		shared := encrypted[tt.depth:]
		decrypted, err := shared.Decrypt(derivedKey)
		if !assert.NoError(t, err, errTag) {
			continue
		}
		assert.Equal(t, tt.path[tt.depth:], decrypted, errTag)
	}
}
