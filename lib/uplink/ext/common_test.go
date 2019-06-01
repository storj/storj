// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
)

// TODO: Start up test planet and call these from bash instead
func TestCCommonTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	runCTest(t, ctx, "common_test.c")
}

func TestGetIDVersion(t *testing.T) {
	var cErr Cchar
	idVersionNumber := storj.LatestIDVersion().Number

	cIDVersion := GetIDVersion(CUint(idVersionNumber), &cErr)
	require.Empty(t, cCharToGoString(cErr))
	require.NotNil(t, cIDVersion)

	assert.Equal(t, idVersionNumber, storj.IDVersionNumber(cIDVersion.number))
}

func TestNewBuffer(t *testing.T) {
	cBufRef := NewBuffer()
	buf, ok := structRefMap.Get(token(cBufRef)).(*bytes.Buffer)
	require.True(t, ok)
	require.NotNil(t, buf)
}

func TestWriteBuffer(t *testing.T) {
	var cErr Cchar
	cBufRef := NewBuffer()

	data := []byte("test write data 123")
	cData := &CBytes{
		bytes: (*CUint8)(&data[0]),
		length: CInt32(len(data)),
	}

	WriteBuffer(cBufRef, cData, &cErr)
	require.Empty(t, cCharToGoString(cErr))

	buf, ok := structRefMap.Get(token(cBufRef)).(*bytes.Buffer)
	require.True(t, ok)
	require.NotNil(t, buf)

	assert.Equal(t, data, buf.Bytes())
}

func TestReadBuffer(t *testing.T) {
	var cErr Cchar

	data := []byte("test read data 123")
	buf := bytes.NewBuffer(data)
	cBufRef := CBufferRef(structRefMap.Add(buf))

	cData := new(CBytes)

	ReadBuffer(cBufRef, cData, &cErr)
	require.Empty(t, cCharToGoString(cErr))
}
