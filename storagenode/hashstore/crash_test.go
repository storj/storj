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
	"storj.io/drpc/drpcsignal"
)

func TestMain(m *testing.M) {
	if dir := os.Getenv("STORJ_HASHSTORE_CRASH_TEST"); dir != "" {
		if err := runCrashServer(context.Background(), dir); err != nil {
			log.Fatalf("%+v", err)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runCrashServer(ctx context.Context, dir string) error {
	db, err := New(ctx, CreateDefaultConfig(TableKind_HashTbl, false), dir, "", nil, nil, nil)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = db.Close() }()

	// launch a goroutine to try to read from stdin so that if the parent
	// process dies, we close the db and eventually exit.
	go func() {
		_, _ = os.Stdin.Read(make([]byte, 1))
		_ = db.Close()
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
	ctx := t.Context()

	// re-exec the process with the STORJ_HASHSTORE_CRASH_TEST env var set to a temp dir.
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = []string{"STORJ_HASHSTORE_CRASH_TEST=" + dir}

	// give a stdin handle to the process so that it can know if the parent process dies.
	cmdStdinR, cmdStdinW, err := os.Pipe()
	assert.NoError(t, err)
	defer func() { _ = cmdStdinR.Close() }()
	defer func() { _ = cmdStdinW.Close() }()
	cmd.Stdin = cmdStdinR

	// capture stdout and stderr so we can read the keys it prints out and any errors.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// start the command and create a signal to keep track of when it finishes.
	assert.NoError(t, cmd.Start())
	var finished drpcsignal.Signal
	go func() { finished.Set(cmd.Wait()) }()

	// wait until we have at least 3 hashtbl files (middle of a compaction).
	for {
		paths := allFiles(t, dir)

		hashtbls := 0
		for _, path := range paths {
			if strings.Contains(path, "hashtbl-") {
				hashtbls++
			}
		}
		if hashtbls >= 3 {
			break
		}

		select {
		case <-finished.Signal():
			t.Fatal("process exited early:", finished.Err())
		case <-time.After(time.Millisecond):
		}
	}

	// kill and wait for the command to exit.
	assert.NoError(t, cmd.Process.Kill())
	<-finished.Signal()

	// grab all of the keys out of stdout.
	var keys []Key
	for line := range bytes.Lines(stdout.Bytes()) {
		key, err := storj.PieceIDFromString(string(line))
		assert.NoError(t, err)
		keys = append(keys, key)
	}
	assert.That(t, len(keys) > 0)

	// open the database and ensure every key that was printed is readable.
	db, err := New(ctx, CreateDefaultConfig(TableKind_HashTbl, false), dir, "", nil, nil, nil)
	assert.NoError(t, err)
	defer assertClose(t, db)

	for _, key := range keys {
		r, err := db.Read(ctx, key)
		assert.NoError(t, err)
		got, err := io.ReadAll(r)
		r.Release()
		assert.NoError(t, err)
		assert.Equal(t, dataFromKey(key), got)
	}
}
