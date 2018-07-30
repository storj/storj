// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package telemetry

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

func TestNewClient_IntervalIsZero(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer s.Close()

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "testapp",
		Instance:    "testinst",
		Interval:    0,
	})

	assert.NotNil(t, client)
	assert.NoError(t, err)
	assert.Equal(t, client.interval, DefaultInterval)
}

func TestNewClient_ApplicationAndArgsAreEmpty(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	oldArgs := os.Args

	defer func() {
		s.Close()
		os.Args = oldArgs
	}()

	os.Args = nil

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "",
		Instance:    "testinst",
		Interval:    0,
	})

	assert.NotNil(t, client)
	assert.NoError(t, err)
	assert.Equal(t, DefaultApplication, client.opts.Application)
}

func TestNewClient_ApplicationIsEmpty(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer s.Close()

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "",
		Instance:    "testinst",
		Interval:    0,
	})

	assert.NotNil(t, client)
	assert.NoError(t, err)
	assert.Equal(t, client.opts.Application, os.Args[0])
}

func TestNewClient_InstanceIsEmpty(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer s.Close()

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "qwe",
		Instance:    "",
		Interval:    0,
	})

	assert.NotNil(t, client)
	assert.NoError(t, err)

	assert.Equal(t, client.opts.InstanceId, []byte(DefaultInstanceID()))
	assert.Equal(t, client.opts.Application, "qwe")
	assert.Equal(t, client.interval, DefaultInterval)
}

func TestNewClient_RegistryIsNil(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer s.Close()

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "qwe",
		Instance:    "",
		Interval:    0,
	})

	assert.NotNil(t, client)
	assert.NoError(t, err)
	assert.Equal(t, client.opts.InstanceId, []byte(DefaultInstanceID()))
	assert.Equal(t, client.opts.Application, "qwe")
	assert.Equal(t, client.interval, DefaultInterval)
	assert.Equal(t, client.opts.Registry, monkit.Default)
}

func TestNewClient_PacketSizeIsZero(t *testing.T) {
	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer s.Close()

	client, err := NewClient(s.Addr(), ClientOpts{
		Application: "qwe",
		Instance:    "",
		Interval:    0,
		PacketSize:  0,
	})

	assert.NotNil(t, client)

	assert.Equal(t, client.opts.InstanceId, []byte(DefaultInstanceID()))
	assert.NoError(t, err)
	assert.Equal(t, client.opts.Application, "qwe")
	assert.Equal(t, client.interval, DefaultInterval)
	assert.Equal(t, client.opts.Registry, monkit.Default)
	assert.Equal(t, client.opts.PacketSize, DefaultPacketSize)
}

func TestRun_ReportNoCalled(t *testing.T) {
	client := &MockClient{}

	ctx := &MockContext{}

	ctx.On("Err").Return(errors.New("")).Once()
	client.On("Report").Times(0)
	client.On("Run", ctx).Once()
	client.Run(ctx)

	ctx.AssertExpectations(t)
}
