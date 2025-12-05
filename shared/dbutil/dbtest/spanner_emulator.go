// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package dbtest

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/execabs"

	"storj.io/storj/shared/processgroup"
)

// StartSpannerEmulator uses bin to start a spanner emulator and passes the connection string to fn.
func StartSpannerEmulator(t TB, host, exe string) string {
	// dirty commands parsing, but should work for most cases
	args := strings.Split(exe, " ")
	if len(args) == 0 {
		t.Fatal("empty executable path")
	}

	bin, err := execabs.LookPath(args[0])
	if err != nil {
		t.Fatalf("binary %q not found: %v", exe, err)
	}
	args = args[1:]

	cmd := execabs.Command(bin, append(args, "--host_port", host+":0")...)
	processgroup.Setup(cmd)

	stdoutWriter := &logWriter{tb: t}
	stderrWriter := &spannerEmulatorHostListener{
		signal: make(chan string, 1),
		tb:     t,
	}

	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start spanner emulator: %w", err)
	}
	t.Cleanup(func() {
		stdoutWriter.stop()
		stderrWriter.stop()

		processgroup.Kill(cmd)
	})

	tick := time.NewTimer(10 * time.Second)
	defer tick.Stop()
	processExit := make(chan struct{})

	go func() {
		defer close(processExit)
		_, _ = cmd.Process.Wait()
	}()

	select {
	case <-processExit:
		t.Fatal("spanner emulator exited unexpectedly")
	case <-tick.C:
		t.Fatal("spanner emulator did not start within 10 seconds")
	case <-t.Context().Done():
		t.Fatal("context canceled while waiting for spanner emulator to start")
	case connstr := <-stderrWriter.signal:
		return "spanner://" + connstr + "?emulator"
	}
	panic("unreachable")
}

type spannerEmulatorHostListener struct {
	tb TB

	mu      sync.Mutex
	stopped bool
	found   bool
	buffer  []byte
	signal  chan string
}

func (log *spannerEmulatorHostListener) stop() {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.stopped = true
}

func (log *spannerEmulatorHostListener) Write(p []byte) (n int, err error) {
	log.mu.Lock()
	defer log.mu.Unlock()

	if log.stopped {
		return 0, io.ErrClosedPipe
	}

	if !log.found {
		log.buffer = append(log.buffer, p...)
		const serverAddress = `Server address: `
		i := bytes.Index(log.buffer, []byte(serverAddress))
		if i >= 0 {
			i += len(serverAddress)
			nl := bytes.IndexByte(log.buffer[i:], '\n')
			if nl >= 0 {
				log.signal <- string(log.buffer[i : i+nl])
				log.found = true
				log.buffer = nil
			}
		}
	}

	log.tb.Log(string(p))

	return len(p), nil
}

type logWriter struct {
	tb TB

	mu      sync.Mutex
	stopped bool
}

func (log *logWriter) stop() {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.stopped = true
}

func (log *logWriter) Write(p []byte) (n int, err error) {
	log.mu.Lock()
	defer log.mu.Unlock()

	if log.stopped {
		return 0, io.ErrClosedPipe
	}

	log.tb.Log(string(p))
	return len(p), nil
}
