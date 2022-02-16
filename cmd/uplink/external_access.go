// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/uplink"
)

func (ex *external) loadAccesses() error {
	if ex.access.accesses != nil {
		return nil
	}

	fh, err := os.Open(ex.AccessInfoFile())
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	var jsonInput struct {
		Default  string
		Accesses map[string]string
	}

	if err := json.NewDecoder(fh).Decode(&jsonInput); err != nil {
		return errs.Wrap(err)
	}

	ex.access.defaultName = jsonInput.Default
	ex.access.accesses = jsonInput.Accesses
	ex.access.loaded = true

	return nil
}

func parseAccessOrPossiblyFile(serializedOrFile string) (*uplink.Access, error) {
	if access, err := uplink.ParseAccess(serializedOrFile); err == nil {
		return access, nil
	}

	serialized, err := ioutil.ReadFile(serializedOrFile)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return uplink.ParseAccess(string(bytes.TrimSpace(serialized)))
}

func (ex *external) OpenAccess(accessName string) (access *uplink.Access, err error) {
	if access, err := parseAccessOrPossiblyFile(accessName); err == nil {
		return access, nil
	}

	accessDefault, accesses, err := ex.GetAccessInfo(true)
	if err != nil {
		return nil, err
	}
	if accessName != "" {
		accessDefault = accessName
	}

	if data, ok := accesses[accessDefault]; ok {
		return uplink.ParseAccess(data)
	}

	// the default was likely a name, so return a potentially nicer message.
	if len(accessDefault) < 20 {
		return nil, errs.New("Cannot find access named %q in saved accesses", accessDefault)
	}
	return nil, errs.New("Unable to get access grant")
}

func (ex *external) GetAccessInfo(required bool) (string, map[string]string, error) {
	if !ex.access.loaded {
		if err := ex.loadAccesses(); err != nil {
			return "", nil, err
		}
		if required && !ex.access.loaded {
			return "", nil, errs.New("No accesses configured. Use 'access save' or 'access create' to create one")
		}
	}

	// return a copy to avoid mutations messing things up
	accesses := make(map[string]string)
	for name, accessData := range ex.access.accesses {
		accesses[name] = accessData
	}

	return ex.access.defaultName, accesses, nil
}

// SaveAccessInfo writes out the access file using the provided values.
func (ex *external) SaveAccessInfo(defaultName string, accesses map[string]string) error {
	// TODO(jeff): write it atomically

	accessFh, err := os.OpenFile(ex.AccessInfoFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = accessFh.Close() }()

	var jsonOutput = struct {
		Default  string
		Accesses map[string]string
	}{
		Default:  defaultName,
		Accesses: accesses,
	}

	data, err := json.MarshalIndent(jsonOutput, "", "\t")
	if err != nil {
		return errs.Wrap(err)
	}

	if _, err := accessFh.Write(data); err != nil {
		return errs.Wrap(err)
	}

	if err := accessFh.Sync(); err != nil {
		return errs.Wrap(err)
	}

	if err := accessFh.Close(); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (ex *external) RequestAccess(ctx context.Context, satelliteAddr, apiKey, passphrase string) (*uplink.Access, error) {
	access, err := uplink.RequestAccessWithPassphrase(ctx, satelliteAddr, apiKey, passphrase)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return access, nil
}

func (ex *external) ExportAccess(ctx clingy.Context, access *uplink.Access, filename string) error {
	serialized, err := access.Serialize()
	if err != nil {
		return errs.Wrap(err)
	}

	// convert to an absolute path, mostly for output purposes.
	filename, err = filepath.Abs(filename)
	if err != nil {
		return errs.Wrap(err)
	}

	// note: we don't use ioutil.WriteFile because we want to pass
	// the O_EXCL flag to ensure we don't overwrite existing files.
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(serialized + "\n"); err != nil {
		return errs.Wrap(err)
	}

	if err := f.Close(); err != nil {
		return errs.Wrap(err)
	}

	fmt.Fprintln(ctx, "Exported access to:", filename)
	return nil
}
