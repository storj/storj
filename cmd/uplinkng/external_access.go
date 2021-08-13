// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

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

func (ex *external) OpenAccess(accessName string) (access *uplink.Access, err error) {
	accessDefault, accesses, err := ex.GetAccessInfo(true)
	if err != nil {
		return nil, err
	}
	if accessName != "" {
		accessDefault = accessName
	}

	if data, ok := accesses[accessDefault]; ok {
		access, err = uplink.ParseAccess(data)
	} else {
		access, err = uplink.ParseAccess(accessDefault)
		// TODO: if this errors then it's probably a name so don't report an error
		// that says "it failed to parse"
	}
	if err != nil {
		return nil, err
	}

	return access, nil
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

func (ex *external) RequestAccess(ctx context.Context, token, passphrase string) (*uplink.Access, error) {
	idx := strings.IndexByte(token, '/')
	if idx == -1 {
		return nil, errs.New("invalid setup token. should be 'satelliteAddress/apiKey'")
	}
	satelliteAddr, apiKey := token[:idx], token[idx+1:]

	access, err := uplink.RequestAccessWithPassphrase(ctx, satelliteAddr, apiKey, passphrase)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return access, nil
}
