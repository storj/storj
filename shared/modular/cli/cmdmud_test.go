// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/mud"
)

type TestComponent struct {
	initialized bool
	running     bool
	closed      bool
	closeDelay  time.Duration
}

func NewTestComponent() *TestComponent {
	return &TestComponent{}
}

func (t *TestComponent) Init(ctx context.Context) error {
	t.initialized = true
	return nil
}

func (t *TestComponent) Run(ctx context.Context) error {
	t.running = true
	<-ctx.Done()
	return nil
}

func (t *TestComponent) Close(ctx context.Context) error {
	fmt.Println("shutting down component", ctx.Err())
	if t.closeDelay > 0 {
		time.Sleep(t.closeDelay)
	}
	t.closed = true
	return nil
}

type TestComponentConfig struct {
	Name string `usage:"component name"`
}

func TestMudCommand_Execute_StackTraceDebug(t *testing.T) {
	t.Run("stack trace created when shutdown exceeds timeout", func(t *testing.T) {
		tempDir := t.TempDir()

		t.Setenv("STORJ_SHUTDOWN_DEBUG_PATH", tempDir)
		t.Setenv("STORJ_SHUTDOWN_TIMEOUT", "1") // 1 second timeout

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ball := mud.NewBall()
		component := NewTestComponent()
		component.closeDelay = 2 * time.Second // Longer than timeout
		mud.Provide[*TestComponent](ball, func() *TestComponent { return component })

		cmd := &MudCommand{
			ball:        ball,
			selector:    mud.All,
			runSelector: mud.All,
			cfg:         &ConfigSupport{},
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Execute(ctx)
		}()

		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
			require.True(t, component.closed)

			// Check if goroutine dump file was created
			files, err := filepath.Glob(filepath.Join(tempDir, "*.goroutines"))
			require.NoError(t, err)
			require.Len(t, files, 1, "Expected exactly one goroutine dump file")

			// Verify the file contains stack trace information
			content, err := os.ReadFile(files[0])
			require.NoError(t, err)
			require.NotEmpty(t, content)

			// Basic check that it looks like a stack trace
			contentStr := string(content)
			require.True(t, strings.Contains(contentStr, "goroutine"), "File should contain goroutine information")

		case <-time.After(10 * time.Second):
			t.Fatal("Execute did not complete within expected time")
		}
	})

	t.Run("no stack trace when debug path not set", func(t *testing.T) {
		t.Setenv("STORJ_SHUTDOWN_TIMEOUT", "1")

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ball := mud.NewBall()
		component := NewTestComponent()
		component.closeDelay = 2 * time.Second
		mud.Provide[*TestComponent](ball, func() *TestComponent { return component })

		cmd := &MudCommand{
			ball:        ball,
			selector:    mud.All,
			runSelector: mud.All,
			cfg:         &ConfigSupport{},
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Execute(ctx)
		}()

		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
			require.True(t, component.closed)
		case <-time.After(10 * time.Second):
			t.Fatal("Execute did not complete within expected time")
		}
	})

	t.Run("no stack trace when shutdown completes quickly", func(t *testing.T) {
		tempDir := t.TempDir()

		t.Setenv("STORJ_SHUTDOWN_DEBUG_PATH", tempDir)
		t.Setenv("STORJ_SHUTDOWN_TIMEOUT", "10")

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ball := mud.NewBall()
		component := NewTestComponent()
		component.closeDelay = 100 * time.Millisecond // Faster than timeout
		mud.Provide[*TestComponent](ball, func() *TestComponent { return component })

		cmd := &MudCommand{
			ball:        ball,
			selector:    mud.All,
			runSelector: mud.All,
			cfg:         &ConfigSupport{},
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Execute(ctx)
		}()

		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
			require.True(t, component.closed)

			// No goroutine dump file should be created
			files, err := filepath.Glob(filepath.Join(tempDir, "*.goroutines"))
			require.NoError(t, err)
			require.Len(t, files, 0, "No goroutine dump file should be created when shutdown is fast")

		case <-time.After(5 * time.Second):
			t.Fatal("Execute did not complete within expected time")
		}
	})
}
