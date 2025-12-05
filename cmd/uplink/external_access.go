// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/uplink"
	"storj.io/uplink/private/access"
)

func (ex *external) loadAccesses() error {
	if ex.access.accesses != nil {
		return nil
	}

	accessInfoFile, err := ex.AccessInfoFile()
	if err != nil {
		return errs.Wrap(err)
	}

	fh, err := os.Open(accessInfoFile)
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

	// older versions may have written out invalid access mapping files
	// so check here and resave if necessary.
	defaultName, ok := checkAccessMapping(jsonInput.Default, jsonInput.Accesses)
	if ok {
		if err := ex.SaveAccessInfo(defaultName, jsonInput.Accesses); err != nil {
			return errs.Wrap(err)
		}
	}

	ex.access.defaultName = jsonInput.Default
	ex.access.accesses = jsonInput.Accesses
	ex.access.loaded = true

	return nil
}

func parseAccessDataOrPossiblyFile(accessDataOrFile string) (*uplink.Access, error) {
	access, parseErr := uplink.ParseAccess(accessDataOrFile)
	if parseErr == nil {
		return access, nil
	}

	accessData, readErr := os.ReadFile(accessDataOrFile)
	if readErr != nil {
		var pathErr *os.PathError
		if errors.As(readErr, &pathErr) {
			readErr = pathErr.Err
		}
		return nil, errs.New("unable to open or parse access: %w", errs.Combine(parseErr, readErr))
	}

	return uplink.ParseAccess(string(bytes.TrimSpace(accessData)))
}

func (ex *external) OpenAccess(accessDesc string) (access *uplink.Access, err error) {
	if access, err := parseAccessDataOrPossiblyFile(accessDesc); err == nil {
		return access, nil
	}

	defaultName, accesses, err := ex.GetAccessInfo(true)
	if err != nil {
		return nil, err
	}
	if accessDesc != "" {
		defaultName = accessDesc
	}

	if accessData, ok := accesses[defaultName]; ok {
		return uplink.ParseAccess(accessData)
	}

	// the default was likely a name, so return a potentially nicer message.
	if len(defaultName) < 20 {
		return nil, errs.New("Cannot find access named %q in saved accesses", defaultName)
	}
	return nil, errs.New("Unable to get access grant")
}

func (ex *external) GetAccessInfo(required bool) (string, map[string]string, error) {
	if !ex.access.loaded {
		if err := ex.loadAccesses(); err != nil {
			return "", nil, err
		}
		if required && !ex.access.loaded {
			return "", nil, errs.New("No accesses configured. Use 'access import' or 'access create' to create one")
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

	accessInfoFile, err := ex.AccessInfoFile()
	if err != nil {
		return errs.Wrap(err)
	}

	accessFh, err := os.OpenFile(accessInfoFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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

func (ex *external) RequestAccess(ctx context.Context, satelliteAddr, apiKey, passphrase string, unencryptedObjectKeys bool) (_ *uplink.Access, err error) {
	defer mon.Task()(&ctx)(&err)

	config := uplink.Config{}
	if unencryptedObjectKeys {
		access.DisableObjectKeyEncryption(&config)
	}
	access, err := config.RequestAccessWithPassphrase(ctx, satelliteAddr, apiKey, passphrase)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return access, nil
}

func (ex *external) ExportAccess(ctx context.Context, access *uplink.Access, filename string) (err error) {
	defer mon.Task()(&ctx)(&err)

	serialized, err := access.Serialize()
	if err != nil {
		return errs.Wrap(err)
	}

	// convert to an absolute path, mostly for output purposes.
	filename, err = filepath.Abs(filename)
	if err != nil {
		return errs.Wrap(err)
	}

	// note: we don't use os.WriteFile because we want to pass
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

	_, _ = fmt.Fprintln(clingy.Stdout(ctx), "Exported access to:", filename)
	return nil
}
