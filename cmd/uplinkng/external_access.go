// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"os"

	"github.com/zeebo/errs"
)

func (ex *external) loadAccesses() error {
	if ex.access.accesses != nil {
		return nil
	}

	fh, err := os.Open(ex.accessFile())
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

func (ex *external) GetAccessInfo(required bool) (string, map[string]string, error) {
	if !ex.access.loaded {
		if err := ex.loadAccesses(); err != nil {
			return "", nil, err
		}
		if required && !ex.access.loaded {
			return "", nil, errs.New("No accesses configured. Use 'access save' to create one")
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

	accessFh, err := os.OpenFile(ex.accessFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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
