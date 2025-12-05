// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"bytes"
	"context"
	"hash/crc32"
	"os"

	"storj.io/common/storj"
)

// localFileService gets encryption keys from local files.
type localFileService struct {
	config Config
}

func newLocalFileService(config Config) *localFileService {
	return &localFileService{
		config: config,
	}
}

// GetKeys gets keys from source.
func (s *localFileService) GetKeys(ctx context.Context) (keys map[int]*storj.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	keys = make(map[int]*storj.Key)

	crc32c := crc32.MakeTable(crc32.Castagnoli)

	for id, k := range s.config.KeyInfos.Values {
		data, err := os.ReadFile(k.SecretVersion)
		if err != nil {
			return nil, Error.New("error reading local key file: %w", err)
		}

		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			return nil, Error.New("empty key data")
		}

		if k.SecretChecksum != int64(crc32.Checksum(data, crc32c)) {
			return nil, Error.New("checksum mismatch")
		}

		keys[id], err = storj.NewKey(data)
		if err != nil {
			return nil, Error.New("could not convert local file key to storj.Key: %w", err)
		}
	}

	return keys, nil
}

// Close closes the service.
func (s *localFileService) Close() error {
	return nil
}
