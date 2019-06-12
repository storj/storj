// +build ignore

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/storj"
)

func TestGetIDVersion(t *testing.T) {
	var cErr Cpchar
	idVersionNumber := storj.LatestIDVersion().Number

	cIDVersion := GetIDVersion(CUint(idVersionNumber), &cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotNil(t, cIDVersion)

	assert.Equal(t, idVersionNumber, storj.IDVersionNumber(cIDVersion.number))
}