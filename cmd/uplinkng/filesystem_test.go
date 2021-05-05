// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

//
// helpers to execute commands for tests
//

func Setup(t *testing.T, opts ...ExecuteOption) State {
	return State{
		opts: opts,
	}
}

type State struct {
	opts []ExecuteOption
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

func (st State) Run(t *testing.T, args ...string) Result {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var stdin bytes.Buffer
	var ops []Operation
	var ran bool

	ok, err := clingy.Environment{
		Name: "uplink-test",
		Args: args,

		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,

		Wrap: func(ctx clingy.Context, cmd clingy.Cmd) error {
			tfs := newTestFilesystem()
			for _, opt := range st.opts {
				if err := opt(ctx, tfs); err != nil {
					return errs.Wrap(err)
				}
			}
			tfs.ops = nil

			if len(tfs.stdin) > 0 {
				_, _ = stdin.WriteString(tfs.stdin)
			}

			if setter, ok := cmd.(interface {
				setTestFilesystem(filesystem)
			}); ok {
				setter.setTestFilesystem(tfs)
			}

			ran = true
			err := cmd.Execute(ctx)
			ops = tfs.ops
			return err
		},
	}.Run(context.Background(), commands)

	if ok && err == nil {
		require.True(t, ran, "no command was executed: %q", args)
	}
	return Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Ok:         ok,
		Err:        err,
		Operations: ops,
	}
}

type ExecuteOption func(ctx clingy.Context, tfs *testFilesystem) error

func WithFile(location string) ExecuteOption {
	return func(ctx clingy.Context, tfs *testFilesystem) error {
		loc, err := parseLocation(location)
		if err != nil {
			return err
		}
		if loc.Remote() {
			tfs.ensureBucket(loc.bucket)
		}
		wh, err := tfs.Create(ctx, loc)
		if err != nil {
			return err
		}
		return wh.Commit()
	}
}

func WithPendingFile(location string) ExecuteOption {
	return func(ctx clingy.Context, tfs *testFilesystem) error {
		loc, err := parseLocation(location)
		if err != nil {
			return err
		}
		if loc.Remote() {
			tfs.ensureBucket(loc.bucket)
		}
		_, err = tfs.Create(ctx, loc)
		return err
	}
}

//
// execution results
//

type Result struct {
	Stdout     string
	Stderr     string
	Ok         bool
	Err        error
	Operations []Operation
}

func (r Result) RequireSuccess(t *testing.T) {
	if !r.Ok {
		errs := parseErrors(r.Stdout)
		require.True(t, r.Ok, "test did not run successfully. errors:\n%s",
			strings.Join(errs, "\n"))
	}
	require.NoError(t, r.Err)
}

func (r Result) RequireFailure(t *testing.T) {
	require.False(t, r.Ok && r.Err == nil, "command ran with no error")
}

func (r Result) RequireStdout(t *testing.T, stdout string) {
	require.Equal(t, trimNewlineSpaces(stdout), trimNewlineSpaces(r.Stdout))
}

func (r Result) RequireStderr(t *testing.T, stderr string) {
	require.Equal(t, trimNewlineSpaces(stderr), trimNewlineSpaces(r.Stderr))
}

func parseErrors(s string) []string {
	lines := strings.Split(s, "\n")
	start := 0
	for i, line := range lines {
		if line == "Errors:" {
			start = i + 1
		} else if len(line) > 0 && line[0] != ' ' {
			return lines[start:i]
		}
	}
	return nil
}

func trimNewlineSpaces(s string) string {
	lines := strings.Split(s, "\n")

	j := 0
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); len(trimmed) > 0 {
			lines[j] = trimmed
			j++
		}
	}
	return strings.Join(lines[:j], "\n")
}

type Operation struct {
	Kind  string
	Loc   string
	Error bool
}

func newOp(kind string, loc Location, err error) Operation {
	return Operation{
		Kind:  kind,
		Loc:   loc.String(),
		Error: err != nil,
	}
}

//
// filesystem
//

type testFilesystem struct {
	stdin   string
	ops     []Operation
	created int64
	files   map[Location]byteFileData
	pending map[Location][]*byteWriteHandle
	buckets map[string]struct{}
}

func newTestFilesystem() *testFilesystem {
	return &testFilesystem{
		files:   make(map[Location]byteFileData),
		pending: make(map[Location][]*byteWriteHandle),
		buckets: make(map[string]struct{}),
	}
}

type byteFileData struct {
	data    []byte
	created int64
}

func (tfs *testFilesystem) ensureBucket(name string) {
	tfs.buckets[name] = struct{}{}
}

func (tfs *testFilesystem) Close() error {
	return nil
}

func (tfs *testFilesystem) Open(ctx clingy.Context, loc Location) (_ readHandle, err error) {
	defer func() { tfs.ops = append(tfs.ops, newOp("open", loc, err)) }()

	bf, ok := tfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist")
	}
	return &byteReadHandle{Buffer: bytes.NewBuffer(bf.data)}, nil
}

func (tfs *testFilesystem) Create(ctx clingy.Context, loc Location) (_ writeHandle, err error) {
	defer func() { tfs.ops = append(tfs.ops, newOp("create", loc, err)) }()

	if loc.Remote() {
		if _, ok := tfs.buckets[loc.bucket]; !ok {
			return nil, errs.New("bucket %q does not exist", loc.bucket)
		}
	}

	tfs.created++
	wh := &byteWriteHandle{
		buf: bytes.NewBuffer(nil),
		loc: loc,
		tfs: tfs,
		cre: tfs.created,
	}

	tfs.pending[loc] = append(tfs.pending[loc], wh)

	return wh, nil
}

func (tfs *testFilesystem) ListObjects(ctx context.Context, prefix Location, recursive bool) (objectIterator, error) {
	var infos []objectInfo
	for loc, bf := range tfs.files {
		if loc.HasPrefix(prefix) {
			infos = append(infos, objectInfo{
				Loc:     loc,
				Created: time.Unix(bf.created, 0),
			})
		}
	}

	sort.Sort(objectInfos(infos))

	if !recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) ListUploads(ctx context.Context, prefix Location, recursive bool) (objectIterator, error) {
	var infos []objectInfo
	for loc, whs := range tfs.pending {
		if loc.HasPrefix(prefix) {
			for _, wh := range whs {
				infos = append(infos, objectInfo{
					Loc:     loc,
					Created: time.Unix(wh.cre, 0),
				})
			}
		}
	}

	sort.Sort(objectInfos(infos))

	if !recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) IsLocalDir(ctx context.Context, loc Location) bool {
	// TODO: implement this

	return false
}

//
// readHandle
//

type byteReadHandle struct {
	*bytes.Buffer
}

func (b *byteReadHandle) Close() error     { return nil }
func (b *byteReadHandle) Info() objectInfo { return objectInfo{} }

//
// writeHandle
//

type byteWriteHandle struct {
	buf  *bytes.Buffer
	loc  Location
	tfs  *testFilesystem
	cre  int64
	done bool
}

func (b *byteWriteHandle) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *byteWriteHandle) Commit() error {
	if err := b.close(); err != nil {
		return err
	}

	b.tfs.ops = append(b.tfs.ops, newOp("commit", b.loc, nil))
	b.tfs.files[b.loc] = byteFileData{
		data:    b.buf.Bytes(),
		created: b.cre,
	}
	return nil
}

func (b *byteWriteHandle) Abort() error {
	if err := b.close(); err != nil {
		return err
	}

	b.tfs.ops = append(b.tfs.ops, newOp("append", b.loc, nil))
	return nil
}

func (b *byteWriteHandle) close() error {
	if b.done {
		return errs.New("already done")
	}
	b.done = true

	handles := b.tfs.pending[b.loc]
	for i, v := range handles {
		if v == b {
			handles = append(handles[:i], handles[i+1:]...)
			break
		}
	}

	if len(handles) > 0 {
		b.tfs.pending[b.loc] = handles
	} else {
		delete(b.tfs.pending, b.loc)
	}

	return nil
}

//
// objectIterator
//

type objectInfoIterator struct {
	infos   []objectInfo
	current objectInfo
}

func (li *objectInfoIterator) Next() bool {
	if len(li.infos) == 0 {
		return false
	}
	li.current, li.infos = li.infos[0], li.infos[1:]
	return true
}

func (li *objectInfoIterator) Err() error {
	return nil
}

func (li *objectInfoIterator) Item() objectInfo {
	return li.current
}

type objectInfos []objectInfo

func (ois objectInfos) Len() int          { return len(ois) }
func (ois objectInfos) Swap(i int, j int) { ois[i], ois[j] = ois[j], ois[i] }
func (ois objectInfos) Less(i int, j int) bool {
	li, lj := ois[i].Loc, ois[j].Loc

	if !li.remote && lj.remote {
		return true
	} else if !lj.remote && li.remote {
		return false
	}

	if li.bucket < lj.bucket {
		return true
	} else if lj.bucket < li.bucket {
		return false
	}

	if li.key < lj.key {
		return true
	} else if lj.key < li.key {
		return false
	}

	if li.path < lj.path {
		return true
	} else if lj.path < li.path {
		return false
	}

	return false
}

func collapseObjectInfos(prefix Location, infos []objectInfo) []objectInfo {
	collapsing := false
	current := ""
	j := 0

	for _, oi := range infos {
		first, ok := oi.Loc.ListKeyName(prefix)
		if ok {
			if collapsing && first == current {
				continue
			}

			collapsing = true
			current = first

			oi.IsPrefix = true
		}

		oi.Loc = oi.Loc.SetKey(first)

		infos[j] = oi
		j++
	}

	return infos[:j]
}
