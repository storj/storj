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

// Node converts the asset to a node.
func (asset *Asset) Node() *Node {
	lookup := map[string]*Node{}
	children := []*Node{}
	for _, child := range asset.Children {
		node := child.Node()
		children = append(children, node)
		lookup[child.Name] = node
	}

	return &Node{
		Name:     asset.Name,
		Size:     int64(len(asset.Data)),
		Mode:     asset.Mode,
		ModTime:  asset.ModTime,
		Data:     asset.Data,
		Children: children,
		Lookup:   lookup,
	}
}

func NewAsset(path string) (*Asset, error) {
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
		Name:     file.Name(),
		Mode:     stat.Mode(),
		ModTime:  stat.ModTime(),
		Children: []*Asset{},
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
		asset, err := NewAsset(filepath.Join(dir, info.Name()))
		if err != nil {
			return err
		}
		asset.Children = append(asset.Children, asset)
	}
	return nil
}
