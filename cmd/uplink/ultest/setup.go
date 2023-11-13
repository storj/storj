// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"io"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
)

// Commands is an alias to refer to a function that builds clingy commands.
type Commands = func(clingy.Commands, ulext.External)

// Setup returns some State that can be run multiple times with different command
// line arguments.
func Setup(cmds Commands, opts ...ExecuteOption) State {
	return State{
		cmds: cmds,
		opts: opts,
	}
}

// State represents some state and environment for a command to execute in.
type State struct {
	cmds Commands
	opts []ExecuteOption
}

// With appends the provided options and returns a new State.
func (st State) With(opts ...ExecuteOption) State {
	st.opts = append([]ExecuteOption(nil), st.opts...)
	st.opts = append(st.opts, opts...)
	return st
}

// Succeed is the same as Run followed by result.RequireSuccess.
func (st State) Succeed(t *testing.T, args ...string) Result {
	result := st.Run(t, args...)
	result.RequireSuccess(t)
	return result
}

// Fail is the same as Run followed by result.RequireFailure.
func (st State) Fail(t *testing.T, args ...string) Result {
	result := st.Run(t, args...)
	result.RequireFailure(t)
	return result
}

// Run executes the command specified by the args and returns a Result.
func (st State) Run(t *testing.T, args ...string) Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var stdin bytes.Buffer
	var ran bool

	ctx := context.Background()
	lfs := ulfs.NewLocal(ulfs.NewLocalBackendMem())
	rfs := newRemoteFilesystem()
	fs := ulfs.NewMixed(lfs, rfs)

	cs := &callbackState{
		fs:  fs,
		rfs: rfs,
	}

	ok, err := clingy.Environment{
		Name: "uplink-test",
		Args: args,

		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,

		Wrap: func(ctx context.Context, cmd clingy.Command) error {
			for _, opt := range st.opts {
				opt.fn(t, ctx, cs)
			}

			if len(cs.stdin) > 0 {
				_, _ = stdin.WriteString(cs.stdin)
			}

			ran = true
			return cmd.Execute(ctx)
		},
	}.Run(ctx, func(cmds clingy.Commands) {
		st.cmds(cmds, newExternal(fs, nil))
	})

	if ok && err == nil {
		require.True(t, ran, "no command was executed: %q", args)
	}

	files := rfs.Files()
	files = gatherLocalFiles(ctx, t, lfs, files)
	sort.Slice(files, func(i, j int) bool { return files[i].less(files[j]) })

	return Result{
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Ok:      ok,
		Err:     err,
		Files:   files,
		Pending: rfs.Pending(),
	}
}

func gatherLocalFiles(ctx context.Context, t *testing.T, fs ulfs.FilesystemLocal, files []File) []File {
	{
		iter, err := fs.List(ctx, "", &ulfs.ListOptions{Recursive: true})
		require.NoError(t, err)
		files = collectIterator(ctx, t, fs, iter, files)
	}
	{
		iter, err := fs.List(ctx, "/", &ulfs.ListOptions{Recursive: true})
		require.NoError(t, err)
		files = collectIterator(ctx, t, fs, iter, files)
	}
	return files
}

func collectIterator(ctx context.Context, t *testing.T, fs ulfs.FilesystemLocal, iter ulfs.ObjectIterator, files []File) []File {
	for iter.Next() {
		func() {
			loc := iter.Item().Loc.Loc()

			mrh, err := fs.Open(ctx, loc)
			require.NoError(t, err)
			defer func() { _ = mrh.Close() }()

			rh, err := mrh.NextPart(ctx, -1)
			require.NoError(t, err)
			defer func() { _ = rh.Close() }()

			data, err := io.ReadAll(rh)
			require.NoError(t, err)
			files = append(files, File{
				Loc:      loc,
				Contents: string(data),
			})
		}()
	}
	require.NoError(t, iter.Err())

	return files
}

type callbackState struct {
	stdin string
	fs    ulfs.Filesystem
	rfs   *remoteFilesystem
}

// ExecuteOption allows one to control the environment that a command executes in.
type ExecuteOption struct {
	fn func(t *testing.T, ctx context.Context, cs *callbackState)
}

// WithFilesystem lets one do arbitrary setup on the filesystem in a callback.
func WithFilesystem(cb func(t *testing.T, ctx context.Context, fs ulfs.Filesystem)) ExecuteOption {
	return ExecuteOption{func(t *testing.T, ctx context.Context, cs *callbackState) {
		cb(t, ctx, cs.fs)
	}}
}

// WithBucket ensures the bucket exists.
func WithBucket(name string) ExecuteOption {
	return ExecuteOption{func(_ *testing.T, _ context.Context, cs *callbackState) {
		cs.rfs.ensureBucket(name)
	}}
}

// WithStdin sets the command to execute with the provided string as standard input.
func WithStdin(stdin string) ExecuteOption {
	return ExecuteOption{func(_ *testing.T, _ context.Context, cs *callbackState) {
		cs.stdin = stdin
	}}
}

// WithFile sets the command to execute with a file created at the given location.
func WithFile(location string, contents ...string) ExecuteOption {
	contents = append([]string(nil), contents...)
	return ExecuteOption{func(t *testing.T, ctx context.Context, cs *callbackState) {
		loc, err := ulloc.Parse(location)
		require.NoError(t, err)

		if bucket, _, ok := loc.RemoteParts(); ok {
			cs.rfs.ensureBucket(bucket)
		}

		mwh, err := cs.fs.Create(ctx, loc, nil)
		require.NoError(t, err)
		defer func() { _ = mwh.Abort(ctx) }()

		wh, err := mwh.NextPart(ctx, -1)
		require.NoError(t, err)
		defer func() { _ = wh.Abort() }()

		for _, content := range contents {
			_, err := wh.Write([]byte(content))
			require.NoError(t, err)
		}
		if len(contents) == 0 {
			_, err := wh.Write([]byte(location))
			require.NoError(t, err)
		}

		require.NoError(t, wh.Commit())
		require.NoError(t, mwh.Commit(ctx))
	}}
}

// WithPendingFile sets the command to execute with a pending upload happening to
// the provided location.
func WithPendingFile(location string) ExecuteOption {
	return ExecuteOption{func(t *testing.T, ctx context.Context, cs *callbackState) {
		loc, err := ulloc.Parse(location)
		require.NoError(t, err)

		if bucket, _, ok := loc.RemoteParts(); ok {
			cs.rfs.ensureBucket(bucket)
		} else {
			t.Fatalf("Invalid pending local file: %s", loc)
		}

		_, err = cs.fs.Create(ctx, loc, nil)
		require.NoError(t, err)
	}}
}
