// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestPrefixWriter(t *testing.T) {
	root := NewPrefixWriter("", ioutil.Discard)
	alpha := root.Prefixed("alpha")
	beta := root.Prefixed("beta")

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
