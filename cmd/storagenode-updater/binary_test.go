// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinaryVersion(t *testing.T) {
	release := `Release build
Version: v1.30.2
Build timestamp: 20 May 21 11:45 CEST
Git commit: 110663703a04739c733644ac57fa48379bb233ed`

	releaseVersion, err := parseVersion([]byte(release))
	require.NoError(t, err)

	require.Equal(t, uint64(1), releaseVersion.Major)
	require.Equal(t, uint64(30), releaseVersion.Minor)
	require.Equal(t, uint64(2), releaseVersion.Patch)

	dev := `Development build
Version: v2024.9.1726198984-4e148b222
Build timestamp: 13 Sep 24 10:41 CEST
Git commit: 4e148b222fd2aef46b1f96057645a7e427871599-dirty
Modified (dirty): true`

	devVersion, err := parseVersion([]byte(dev))
	require.NoError(t, err)

	require.Equal(t, uint64(2024), devVersion.Major)
	require.Equal(t, uint64(9), devVersion.Minor)
	require.Equal(t, uint64(1726198984), devVersion.Patch)

	require.Equal(t, -1, releaseVersion.Compare(devVersion))
}
