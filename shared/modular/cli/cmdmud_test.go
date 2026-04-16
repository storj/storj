// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/modular"
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

// OrderTracker tracks the order of init, run, and close calls across components.
type OrderTracker struct {
	mu  sync.Mutex
	seq []string
}

func NewOrderTracker() *OrderTracker {
	return &OrderTracker{}
}

func (o *OrderTracker) Record(event string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.seq = append(o.seq, event)
}

func (o *OrderTracker) Seq() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string{}, o.seq...)
}

func (o *OrderTracker) waitFor(duration time.Duration, target string) {
	deadline := time.Now().Add(duration)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return
		}

		o.mu.Lock()
		for _, event := range o.seq {
			if event == target {
				o.mu.Unlock()
				return
			}
		}
		o.mu.Unlock()
		<-ticker.C
	}
}

// ComponentC is a normal component (no RunEarly tag).
type ComponentC struct {
	tracker *OrderTracker
}

func NewComponentC(tracker *OrderTracker) *ComponentC {
	tracker.Record("InitC")
	return &ComponentC{tracker: tracker}
}

func (c *ComponentC) Run(ctx context.Context) error {
	c.tracker.Record("RunC")
	<-ctx.Done()
	return nil
}

func (c *ComponentC) Close(ctx context.Context) error {
	c.tracker.Record("CloseC")
	return nil
}

// ComponentB is a RunEarly component that depends on ComponentC.
type ComponentB struct {
	tracker *OrderTracker
	depC    *ComponentC
}

func NewComponentB(tracker *OrderTracker, c *ComponentC) *ComponentB {
	tracker.Record("InitB")
	return &ComponentB{tracker: tracker, depC: c}
}

func (b *ComponentB) Run(ctx context.Context) error {
	b.tracker.Record("RunB")
	return nil
}

func (b *ComponentB) Close(ctx context.Context) error {
	b.tracker.Record("CloseB")
	return nil
}

// ComponentA is a normal component that depends on ComponentB (RunEarly).
type ComponentA struct {
	tracker *OrderTracker
	depB    *ComponentB
}

func NewComponentA(tracker *OrderTracker, b *ComponentB) *ComponentA {
	// if we do everything right, RunB should be started earlier earlier (but run is async, so we need to wait)
	tracker.waitFor(10*time.Second, "RunB")
	tracker.waitFor(10*time.Second, "RunC")
	time.Sleep(1 * time.Second)
	tracker.Record("InitA")
	return &ComponentA{tracker: tracker, depB: b}
}

func (a *ComponentA) Run(ctx context.Context) error {
	a.tracker.Record("RunA")
	return nil
}

func (a *ComponentA) Close(ctx context.Context) error {
	a.tracker.Record("CloseA")
	return nil
}

// setupRunEarlyTest creates a ball with components A -> B (RunEarly) -> C and returns the tracker.
func setupRunEarlyTest() (*mud.Ball, *OrderTracker) {
	tracker := NewOrderTracker()
	ball := mud.NewBall()

	mud.Supply[*OrderTracker](ball, tracker)
	mud.Provide[*ComponentA](ball, NewComponentA)
	mud.Provide[*ComponentB](ball, NewComponentB)
	mud.Provide[*ComponentC](ball, NewComponentC)
	mud.Tag[*ComponentB, modular.RunEarly](ball, modular.RunEarly{})

	return ball, tracker
}

// runMudCommand executes a MudCommand with the given selector and waits for completion.
func runMudCommand(t *testing.T, ball *mud.Ball, selector mud.ComponentSelector) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())

	cmd := &MudCommand{
		ball:        ball,
		selector:    selector,
		runSelector: selector,
		cfg:         &ConfigSupport{},
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not complete within expected time")
	}
}

// TestRunEarlyInitializationOrder tests that RunEarly components and their dependencies
// are initialized and run before other components.
// Scenario: A (normal) -> B (RunEarly) -> C (normal)
// Expected order:
// Phase 1: Init C, Init B (RunEarly dependencies)
// Phase 2: Run B (RunEarly)
// Phase 3: Init A (remaining)
// Phase 4: Run C, Run A (remaining).
// indexOfEvent returns the index of an event in the sequence, or -1 if not found.
func indexOfEvent(seq []string, event string) int {
	for i, e := range seq {
		if e == event {
			return i
		}
	}
	return -1
}

func TestRunEarlyInitializationOrder(t *testing.T) {
	t.Run("select all components", func(t *testing.T) {
		ball, tracker := setupRunEarlyTest()

		runMudCommand(t, ball, mud.All)

		seq := tracker.Seq()
		t.Logf("Sequence: %v", seq)

		// Verify init order: C before B before A (dependency order)
		require.Less(t, indexOfEvent(seq, "InitC"), indexOfEvent(seq, "InitB"), "InitC should be before InitB")
		require.Less(t, indexOfEvent(seq, "InitB"), indexOfEvent(seq, "InitA"), "InitB should be before InitA")

		// Verify RunEarly: RunB and RunC should happen before InitA
		// This is the key RunEarly guarantee - early components run before other components initialize
		require.Less(t, indexOfEvent(seq, "RunB"), indexOfEvent(seq, "InitA"), "RunB should be before InitA (RunEarly)")
		require.Less(t, indexOfEvent(seq, "RunC"), indexOfEvent(seq, "InitA"), "RunC should be before InitA (RunEarly dependency)")

		// Verify RunB happens before RunA (A waits for B)
		require.Less(t, indexOfEvent(seq, "RunB"), indexOfEvent(seq, "RunA"), "RunB should be before RunA")

		// Verify all components are initialized before their Run is called
		require.Less(t, indexOfEvent(seq, "InitA"), indexOfEvent(seq, "RunA"), "InitA should be before RunA")
		require.Less(t, indexOfEvent(seq, "InitB"), indexOfEvent(seq, "RunB"), "InitB should be before RunB")
		require.Less(t, indexOfEvent(seq, "InitC"), indexOfEvent(seq, "RunC"), "InitC should be before RunC")

		// Verify close order is reverse of init (A, B, C)
		require.Less(t, indexOfEvent(seq, "CloseA"), indexOfEvent(seq, "CloseB"), "CloseA should be before CloseB")
		require.Less(t, indexOfEvent(seq, "CloseB"), indexOfEvent(seq, "CloseC"), "CloseB should be before CloseC")
	})

	t.Run("select ComponentA only", func(t *testing.T) {
		// Selecting A should also init/run B (RunEarly dependency) and C (B's dependency)
		ball, tracker := setupRunEarlyTest()

		runMudCommand(t, ball, mud.Select[*ComponentA](ball))

		seq := tracker.Seq()
		t.Logf("Sequence: %v", seq)

		// Verify all three components are present
		require.Contains(t, seq, "InitA")
		require.Contains(t, seq, "InitB")
		require.Contains(t, seq, "InitC")
		require.Contains(t, seq, "RunA")
		require.Contains(t, seq, "RunB")
		require.Contains(t, seq, "RunC")

		// Verify RunEarly: RunB and RunC should happen before InitA
		require.Less(t, indexOfEvent(seq, "RunB"), indexOfEvent(seq, "InitA"), "RunB should be before InitA (RunEarly)")
		require.Less(t, indexOfEvent(seq, "RunC"), indexOfEvent(seq, "InitA"), "RunC should be before InitA (RunEarly dependency)")

		// Verify RunB happens before RunA (A waits for B)
		require.Less(t, indexOfEvent(seq, "RunB"), indexOfEvent(seq, "RunA"), "RunB should be before RunA")
	})

	t.Run("select ComponentB only", func(t *testing.T) {
		// Selecting B (RunEarly) should init/run B and C (B's dependency)
		// A should NOT be initialized or run
		ball, tracker := setupRunEarlyTest()

		runMudCommand(t, ball, mud.Select[*ComponentB](ball))

		seq := tracker.Seq()
		t.Logf("Sequence: %v", seq)

		// Verify B and C are present, A is not
		require.Contains(t, seq, "InitB")
		require.Contains(t, seq, "InitC")
		require.Contains(t, seq, "RunB")
		require.Contains(t, seq, "RunC")
		require.NotContains(t, seq, "InitA", "A should not be initialized")
		require.NotContains(t, seq, "RunA", "A should not run")
	})

	t.Run("select ComponentC only", func(t *testing.T) {
		// Selecting C should only init/run C
		// B and A should NOT be initialized or run
		ball, tracker := setupRunEarlyTest()

		runMudCommand(t, ball, mud.Select[*ComponentC](ball))

		seq := tracker.Seq()
		t.Logf("Sequence: %v", seq)

		// Verify only C is present
		require.Equal(t, []string{"InitC", "RunC", "CloseC"}, seq)
	})
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
