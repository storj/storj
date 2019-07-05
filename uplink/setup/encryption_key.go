// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package setup

import (
	"os"

	"github.com/zeebo/errs"
)

// SaveEncryptionKey generates a Storj key from the inputKey and save it into a
// new file created in filepath.
func SaveEncryptionKey(inputKey string, filepath string) error {
	switch {
	case len(inputKey) == 0:
		return Error.New("inputKey is empty")
	case filepath == "":
		return Error.New("filepath is empty")
	}

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return Error.New("directory path doesn't exist. %+v", err)
		}

		if os.IsExist(err) {
			return Error.New("file key already exists. %+v", err)
		}

		return Error.Wrap(err)
	}

	defer func() {
		err = Error.Wrap(errs.Combine(err, file.Close()))
	}()

	_, err = file.Write([]byte(inputKey))
	return err
}
