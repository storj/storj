// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
)

// Asset is for conveniently defining a tree of data.
type Asset struct {
	Name     string
	Mode     os.FileMode
	ModTime  time.Time
	Data     []byte
	Children []*Asset
}

// NewDir loads an asset directory from filesystem.
func NewDir(path string) (*Asset, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	asset, err := New(abspath)
	if err != nil {
		return nil, err
	}
	asset.Name = ""
	return asset, nil
}

// New loads an asset from filesystem.
func New(path string) (*Asset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	asset := &Asset{
		Name:    stat.Name(),
		Mode:    stat.Mode(),
		ModTime: stat.ModTime(),
	}

	if stat.IsDir() {
		children, err := file.Readdir(-1)
		if err != nil {
			return nil, err
		}
		asset.addFiles(path, children)
	} else {
		asset.Data, err = ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}
	}

	return asset, nil
}

// addFiles adds all nested files to asset
func (asset *Asset) addFiles(dir string, infos []os.FileInfo) error {
	for _, info := range infos {
		child, err := New(filepath.Join(dir, info.Name()))
		if err != nil {
			return err
		}
		asset.Children = append(asset.Children, child)
	}
	return nil
}
