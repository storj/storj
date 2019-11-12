// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build windows

package main_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/internal/testcontext"
)

func createTestService(ctx *testcontext.Context, t *testing.T, name, binPath string) (cleanup func()) {
	manager, err := mgr.Connect()
	require.NoError(t, err)

	service, err := manager.OpenService(name)
	if err == nil {
		err = service.Close()
		err = errs.Combine(err, manager.Disconnect())
		assert.NoError(t, err)
		t.Fatalf("service \"%s\" already exists", name)
	}

	config := mgr.Config{
		DisplayName: name,
	}

	args := []string{
		"run",
		"--config-dir", ctx.Dir(),
		"--service-name", name,
		"--binary-location", ctx.File("fake", "storagenode.exe"),
		"--check-interval", "30s",
		"--identity.cert-path", ctx.File("identity", "identity.cert"),
		"--identity.key-path", ctx.File("identity", "identity.key"),
	}
	service, err = manager.CreateService(name, binPath, config, args...)
	if !assert.NoError(t, err) {
		err = errs.Combine(service.Close())
		err = errs.Combine(manager.Disconnect())
		t.Fatal("unable to create service", err)
	}

	err = service.Start()
	if !assert.NoError(t, err) {
		err = errs.Combine(service.Delete())
		err = errs.Combine(service.Close())
		err = errs.Combine(manager.Disconnect())
		t.Fatal("unable to start service", err)
	}

	return func() {
		status, err := service.Control(svc.Stop)
		for status.State != svc.Stopped {
			status, err = service.Query()
			if err != nil {
				break
			}
			if !sync2.Sleep(ctx, 100*time.Millisecond) {
				break
			}
		}

		require.NoError(t, errs.Combine(
			err,
			service.Delete(),
			service.Close(),
			manager.Disconnect(),
		))
	}
}
