// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

func TestMain(m *testing.M) {
	if os.Getenv("STORJ_HASHSTORE_CRASH_TEST") != "" {
		if err := runCrashServer(context.Background()); err != nil {
			log.Fatalf("%+v", err)
		}
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func runCrashServer(ctx context.Context) error {
	tmpDir, err := os.MkdirTemp("", "hashstore-crash-test-*")
	if err != nil {
		return errs.Wrap(err)
	}
	fmt.Println(tmpDir)

	db, err := New(ctx, tmpDir, "", nil, nil, nil)
	if err != nil {
		return errs.Wrap(err)
	}
	defer db.Close()

	// launch a goroutine to try to read from stdin so that if the parent
	// process dies, we close the db and eventually exit.
	go func() {
		_, _ = os.Stdin.Read(make([]byte, 1))
		db.Close()
	}()

	mu := new(sync.Mutex) // protects writing to stdout

	create := func() error {
		key := newKey()

		wr, err := db.Create(ctx, key, time.Time{})
		if err != nil {
			return errs.Wrap(err)
		}
		defer wr.Cancel()

		if _, err := wr.Write(dataFromKey(key)); err != nil {
			return errs.Wrap(err)
		}

		if err := wr.Close(); err != nil {
			return errs.Wrap(err)
		}

		mu.Lock()
		defer mu.Unlock()

		if _, err := fmt.Println(key.String()); err != nil {
			return errs.Wrap(err)
		}

		return nil
	}

	spam := func() error {
		for {
			if err := create(); err != nil {
				return err
			}
		}
	}

	errCh := make(chan error)
	for range runtime.GOMAXPROCS(-1) {
		go func() { errCh <- spam() }()
	}
	return <-errCh
}

func TestCorrectDuringCrashes(t *testing.T) {
	ctx := context.Background()

	// re-exec the process with the STORJ_HASHSTORE_CRASH_TEST env var set
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "STORJ_HASHSTORE_CRASH_TEST=server")

	// give a stdin handle to the process so that it can know if the parent process dies.
	cmdStdinR, cmdStdinW, err := os.Pipe()
	assert.NoError(t, err)
	defer func() { _ = cmdStdinR.Close() }()
	defer func() { _ = cmdStdinW.Close() }()
	cmd.Stdin = cmdStdinR

	// give a stdout handle so we can read the keys it prints out.
	var stdout lockedBuffer
	cmd.Stdout = &stdout

	// give a stderr handle so we can look at any errors it prints out.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// start the command.
	assert.NoError(t, cmd.Start())

	// wait until the directory is printed.
	var dir string
	for {
		if prefix, _, ok := strings.Cut(stdout.String(), "\n"); ok {
			dir = prefix
			break
		}

		time.Sleep(time.Millisecond)
	}

	// ensure the directory is what we expect and clean it up when we're done.
	assert.That(t, strings.HasPrefix(dir, os.TempDir()))
	defer func() { _ = os.RemoveAll(dir) }()

	// wait until we have at least 3 hashtbl files (middle of a compaction).
	for {
		paths, err := allFiles(dir)
		assert.NoError(t, err)

		hashtbls := 0
		for _, path := range paths {
			if strings.Contains(path, "meta/hashtbl-") {
				hashtbls++
			}
		}
		if hashtbls >= 3 {
			break
		}

		time.Sleep(time.Millisecond)
	}

	// kill and wait for the command to exit.
	assert.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()

	// grab all of the keys out of stdout.
	var keys []Key
	for _, line := range strings.Split(stdout.String(), "\n")[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}

		key, err := storj.PieceIDFromString(line)
		assert.NoError(t, err)
		keys = append(keys, key)
	}
	assert.That(t, len(keys) > 0)

	// open the database and ensure every key that was printed is readable.
	db, err := New(ctx, dir, "", nil, nil, nil)
	assert.NoError(t, err)
	defer db.Close()

	for _, key := range keys {
		r, err := db.Read(ctx, key)
		assert.NoError(t, err)
		got, err := io.ReadAll(r)
		r.Release()
		assert.NoError(t, err)
		assert.Equal(t, dataFromKey(key), got)
	}
}

type lockedBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (a *lockedBuffer) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.buf.String()
}

func (a *lockedBuffer) Write(p []byte) (n int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.buf.Write(p)
}
