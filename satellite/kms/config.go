// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package kms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

// Config is a configuration struct for secret management Service.
type Config struct {
	Provider         string   `help:"the provider of the passphrase encryption keys: 'gsm' for google, 'local' for a local file" default:"gsm"`
	KeyInfos         KeyInfos `help:"semicolon-separated key-id:version,checksum. With 'gsm' provider, version is the resource name. With 'local', it is the file path. Checksum is the integer crc32c checksum of the key data" default:""`
	DefaultMasterKey int      `help:"the key ID to use for passphrase encryption." default:"1"`
	TestMasterKey    string   `help:"[DEPRECATED] For testing, use --kms.mock-client and --kms.key-infos. A fake master key to be used for the purpose of testing." releaseDefault:"" devDefault:"test-master-key" hidden:"true"`
	MockClient       bool     `help:"whether to use mock google secret manager service." releaseDefault:"false" devDefault:"false" testDefault:"true" hidden:"true"`
}

// KeyInfo contains the location and checksum of a key.
type KeyInfo struct {
	SecretVersion  string
	SecretChecksum int64
}

// Ensure that KeyInfos implements pflag.Value.
var _ pflag.Value = (*KeyInfos)(nil)

// KeyInfos is a map of key IDs to key infos.
type KeyInfos struct {
	Values map[int]KeyInfo
}

// Type returns the type of the pflag.Value.
func (KeyInfos) Type() string { return "kms.Keys" }

func (ki *KeyInfos) String() string {
	var s strings.Builder
	i := 0
	for k, v := range ki.Values {
		if i > 0 {
			s.WriteString(";")
		}
		_, _ = fmt.Fprintf(&s, "%d:%s,%d", k, v.SecretVersion, v.SecretChecksum)
		i++
	}
	return s.String()
}

// Set sets the list of keys to the parsed string.
func (ki *KeyInfos) Set(s string) error {
	keyInfosMap := make(map[int]KeyInfo)
	for _, keyStr := range strings.Split(s, ";") {
		if keyStr == "" {
			continue
		}

		info := strings.Split(keyStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid key (expected format key-id:version,checksum, got %s)", keyStr)
		}

		idStr := strings.TrimSpace(info[0])
		if idStr == "" {
			return Error.New("key id must not be empty")
		}

		idInt, err := strconv.Atoi(idStr)
		if err != nil {
			return Error.New("Invalid key. Unable to convert string to integer: %w", err)
		}

		keyID := idInt

		valuesStr := info[1]
		values := strings.Split(valuesStr, ",")
		if len(values) != 2 {
			return Error.New("Invalid values (expected format version,checksum, got %s)", valuesStr)
		}

		checksum, err := strconv.Atoi(values[1])
		if err != nil {
			return Error.New("Invalid checksum: %s", err)
		}

		if _, ok := keyInfosMap[keyID]; ok {
			return Error.New("key ID duplicate found. Key IDs must be unique: %d", keyID)
		}

		keyInfosMap[keyID] = KeyInfo{
			SecretVersion:  values[0],
			SecretChecksum: int64(checksum),
		}
	}
	ki.Values = keyInfosMap
	return nil
}
