// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestPrefixWriter(t *testing.T) {
	root := NewPrefixWriter("", storjSimMaxLineLen, io.Discard)
	alpha := root.Prefixed("alpha")
	defer func() { _ = alpha.Flush() }()
	beta := root.Prefixed("beta")
	defer func() { _ = beta.Flush() }()

	var group errgroup.Group
	defer func() {
		require.NoError(t, group.Wait())
	}()

	group.Go(func() error {
		_, err := alpha.Write([]byte{1, 2, 3})
		return err
	})
	group.Go(func() error {
		_, err := alpha.Write([]byte{3, 2, 1})
		return err
	})
	group.Go(func() error {
		_, err := beta.Write([]byte{1, 2, 3})
		return err
	})
}

func TestPrefixWriterWithLongLine(t *testing.T) {
	const (
		maxLineLen  = 46
		inputString = "It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity,"
		prefix      = "dickens"
		nodeID      = "TWO-CITIES"
	)

	dst := &bytes.Buffer{}
	w := NewPrefixWriter(prefix, maxLineLen, dst)
	w.nowFunc = func() time.Time { return time.Unix(1599750982, 123456789).UTC() }

	_, err := w.Write([]byte("Hi. Node " + nodeID + " started\n"))
	require.NoError(t, err)

	n, err := w.Write([]byte(inputString))
	require.NoError(t, err)
	require.Equal(t, len(inputString), n)

	// then write the newline separately
	n, err = w.Write([]byte("\n"))
	require.NoError(t, err)
	require.Equal(t, 1, n)

	expected := `
dickens TWO-CITIES 15:16:22.123 | Hi. Node TWO-CITIES started
dickens TWO-CITIES 15:16:22.123 | It was the best of times, it was the worst of
                                |  times, it was the age of wisdom, it was the
                                |  age of foolishness, it was the epoch of
dickens TWO-CITIES 15:16:22.123 |  belief, it was the epoch of incredulity,
`
	require.Equal(t, strings.TrimLeft(expected, "\n"), dst.String())
}

func TestPrefixWriterWithLongLineAndNewline(t *testing.T) {
	const (
		maxLineLen  = 46
		inputString = "It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity,\n"
		prefix      = "dickens"
		nodeID      = "TWO-CITIES"
	)

	dst := &bytes.Buffer{}
	w := NewPrefixWriter(prefix, maxLineLen, dst)
	w.nowFunc = func() time.Time { return time.Unix(1599750982, 123456789).UTC() }

	_, err := w.Write([]byte("Hi. Node " + nodeID + " started\n"))
	require.NoError(t, err)

	n, err := w.Write([]byte(inputString))
	require.NoError(t, err)
	require.Equal(t, len(inputString), n)

	expected := `
dickens TWO-CITIES 15:16:22.123 | Hi. Node TWO-CITIES started
dickens TWO-CITIES 15:16:22.123 | It was the best of times, it was the worst of
                                |  times, it was the age of wisdom, it was the
                                |  age of foolishness, it was the epoch of
                                |  belief, it was the epoch of incredulity,
`
	require.Equal(t, strings.TrimLeft(expected, "\n"), dst.String())
}
