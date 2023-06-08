// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeebo/errs"
)

// LocalBackendMem implements LocalBackend with memory backed files.
type LocalBackendMem struct {
	root *memDir
	cwd  *memDir
}

// NewLocalBackendMem creates a new LocalBackendMem.
func NewLocalBackendMem() *LocalBackendMem {
	return &LocalBackendMem{
		root: newMemDir("/"),
		cwd:  newMemDir(""),
	}
}

func (l *LocalBackendMem) openRoot(name string) (string, *memDir) {
	if strings.HasPrefix(name, string(filepath.Separator)) {
		return name, l.root
	} else if name == "." {
		return name[1:], l.cwd
	} else if strings.HasPrefix(name, "./") {
		return name[2:], l.cwd
	}
	return name, l.cwd
}

func (l *LocalBackendMem) openParent(name string) (*memDir, string, error) {
	dir := filepath.Dir(name)

	fh, err := l.Open(dir)
	if err != nil {
		return nil, "", err
	}

	md, ok := fh.(*memDir)
	if !ok {
		return nil, "", errs.New("parent not a directory: %q", dir)
	}
	return md, filepath.Base(name), nil
}

// Create creates a new file for the given name.
func (l *LocalBackendMem) Create(name string) (LocalBackendFile, error) {
	name = filepath.Clean(name)

	md, base, err := l.openParent(name)
	if err != nil {
		return nil, err
	}
	if fh, ok := md.children[base]; ok {
		if _, ok := fh.(*memDir); ok {
			return nil, errs.New("file already exists: %q", name)
		}
	}
	mf := newMemFile(name)
	md.children[base] = mf
	return mf, nil
}

// MkdirAll recursively creates directories to make name a directory.
func (l *LocalBackendMem) MkdirAll(name string, perm os.FileMode) error {
	name = filepath.Clean(name)

	name, root := l.openRoot(name)
	return iterateComponents(name, func(name, ent string) error {
		fh, ok := root.children[ent]
		if !ok {
			fh = newMemDir(name)
			root.children[ent] = fh
		}
		md, ok := fh.(*memDir)
		if !ok {
			return errs.New("file already exists: %q", name)
		}
		root = md
		return nil
	})
}

// Open opens the file with the given name.
func (l *LocalBackendMem) Open(name string) (LocalBackendFile, error) {
	name = filepath.Clean(name)

	var root LocalBackendFile
	name, root = l.openRoot(name)
	err := iterateComponents(name, func(name, ent string) error {
		md, ok := root.(*memDir)
		if !ok {
			return errs.New("not a directory: %q", name)
		}
		fh, ok := md.children[ent]
		if !ok {
			return os.ErrNotExist
		}
		root = fh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return root, nil
}

// Remove deletes the file with the given name.
func (l *LocalBackendMem) Remove(name string) error {
	name = filepath.Clean(name)

	md, base, err := l.openParent(name)
	if err != nil {
		return err
	}
	if _, ok := md.children[base]; !ok {
		return errs.New("file does not exists: %q", name)
	}
	delete(md.children, base)
	return nil
}

// Rename causes the file at oldname to be moved to newname.
func (l *LocalBackendMem) Rename(oldname, newname string) error {
	oldname = filepath.Clean(oldname)
	newname = filepath.Clean(newname)

	omd, obase, err := l.openParent(oldname)
	if err != nil {
		return err
	}
	nmd, nbase, err := l.openParent(newname)
	if err != nil {
		return err
	}

	f, ok := omd.children[obase]
	if !ok {
		return os.ErrNotExist
	}

	switch f := f.(type) {
	case *memFile:
		f.name = newname
	case *memDir:
		f.name = newname
	}

	nmd.children[nbase] = f
	delete(omd.children, obase)

	return nil
}

// Stat returns file info for the given name.
func (l *LocalBackendMem) Stat(name string) (os.FileInfo, error) {
	fh, err := l.Open(name)
	if err != nil {
		return nil, err
	}
	return fh.Stat()
}

//
// memFile
//

type memFile struct {
	name string
	buf  []byte
}

func newMemFile(name string) *memFile {
	return &memFile{
		name: name,
	}
}

func (mf *memFile) String() string { return fmt.Sprintf("File[%q]", mf.name) }

func (mf *memFile) Name() string { return mf.name }
func (mf *memFile) Close() error { return nil }

func (mf *memFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(mf.buf)) {
		return 0, io.EOF
	}
	return copy(p, mf.buf[off:]), nil
}

func (mf *memFile) WriteAt(p []byte, off int64) (int, error) {
	if delta := (off + int64(len(p))) - int64(len(mf.buf)); delta > 0 {
		mf.buf = append(mf.buf, make([]byte, delta)...)
	}
	return copy(mf.buf[off:], p), nil
}

func (mf *memFile) Stat() (os.FileInfo, error) {
	return (*memFileInfo)(mf), nil
}

func (mf *memFile) Readdir(n int) ([]os.FileInfo, error) {
	return nil, errs.New("readdir on regular file")
}

type memFileInfo memFile

var _ os.FileInfo = (*memFileInfo)(nil)

func (mfi *memFileInfo) Name() string {
	return filepath.Base((*memFile)(mfi).name)
}

func (mfi *memFileInfo) Size() int64        { return int64(len((*memFile)(mfi).buf)) }
func (mfi *memFileInfo) Mode() fs.FileMode  { return 0777 }
func (mfi *memFileInfo) ModTime() time.Time { return time.Time{} }
func (mfi *memFileInfo) IsDir() bool        { return false }
func (mfi *memFileInfo) Sys() interface{}   { return nil }

//
// memDir
//

type memDir struct {
	name     string
	children map[string]LocalBackendFile
}

func newMemDir(name string) *memDir {
	return &memDir{
		name:     name,
		children: make(map[string]LocalBackendFile),
	}
}

var _ LocalBackendFile = (*memDir)(nil)

func (md *memDir) String() string { return fmt.Sprintf("Dir[%q, %v]", md.name, md.children) }

func (md *memDir) Name() string { return md.name }
func (md *memDir) Close() error { return nil }

func (md *memDir) ReadAt(p []byte, off int64) (int, error) {
	return 0, errs.New("readat on directory")
}

func (md *memDir) WriteAt(p []byte, off int64) (int, error) {
	return 0, errs.New("writeat on directory")
}

func (md *memDir) Stat() (os.FileInfo, error) {
	return (*memDirInfo)(md), nil
}

func (md *memDir) Readdir(n int) ([]os.FileInfo, error) {
	if n != -1 {
		return nil, errs.New("can only read all entries")
	}
	out := make([]os.FileInfo, 0, len(md.children))
	for _, child := range md.children {
		info, _ := child.Stat()
		out = append(out, info)
	}
	return out, nil
}

type memDirInfo memDir

var _ os.FileInfo = (*memDirInfo)(nil)

func (dfi *memDirInfo) Name() string {
	return filepath.Base((*memDir)(dfi).name)
}

func (dfi *memDirInfo) Size() int64        { return 0 }
func (dfi *memDirInfo) Mode() fs.FileMode  { return 0777 }
func (dfi *memDirInfo) ModTime() time.Time { return time.Time{} }
func (dfi *memDirInfo) IsDir() bool        { return true }
func (dfi *memDirInfo) Sys() interface{}   { return nil }

//
// helpers
//

func iterateComponents(name string, cb func(name, ent string) error) error {
	i := 0
	for i < len(name) {
		part := strings.IndexByte(name[i:], filepath.Separator)
		if part == -1 {
			return cb(name, name[i:])
		}
		if err := cb(name[:i+part], name[i:i+part]); err != nil {
			return err
		}
		i += part + 1
	}
	return nil
}
