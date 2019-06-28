// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package asset implements asset embedding via implementing http.FileSystem interface.
//
// To use the package you would define:
//
//     //go:generate go run ../internal/asset/generate/main.go -pkg main -dir ../../web/bootstrap -var embeddedAssets -out console.resource.go
//     var embeddedAssets http.Dir
//
// This will generate a new "console.resource.go" which contains the content of "../../web/bootstrap".
//
// In the program initialization you can select based on whether the embedded resources exist or not:
//
//     var assets http.FileSystem
//     if *staticAssetDirectory != "" {
//         assets = http.Dir(*staticAssetDirectory)
//     } else if embeddedAssets == nil {
//         assets = embeddedAssets
//     } else {
//         assets = http.Dir(defaultAssetLocation)
//     }
//
// Then write the service in terms of http.FileSystem, which hides the actual thing used for loading.
//
package asset

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
)

// Asset describes a tree of asset files and directories.
type Asset struct {
	Name     string
	Mode     os.FileMode
	ModTime  time.Time
	Data     []byte
	Children []*Asset
}

// ReadDir loads an asset directory from filesystem.
func ReadDir(path string) (*Asset, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	asset, err := ReadFile(abspath)
	if err != nil {
		return nil, err
	}
	asset.Name = ""
	return asset, nil
}

// ReadFile loads an asset from filesystem.
func ReadFile(path string) (*Asset, error) {
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
		child, err := ReadFile(filepath.Join(dir, info.Name()))
		if err != nil {
			return err
		}
		asset.Children = append(asset.Children, child)
	}
	return nil
}
