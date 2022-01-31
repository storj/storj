// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package simulate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/mailservice/simulate"
)

func TestFindLinks(t *testing.T) {
	data := `
		<a href="link1" data-simulate>
		<A HREF="link2" data-simulate>
		<a href="link3">
		<a href>
		<a data-simulate>
	`

	clicker := simulate.LinkClicker{MarkerAttribute: "data-simulate"}
	require.ElementsMatch(t, []string{"link1", "link2"}, clicker.FindLinks(data))

	clicker.MarkerAttribute = ""
	require.ElementsMatch(t, []string{"link1", "link2", "link3"}, clicker.FindLinks(data))
}
