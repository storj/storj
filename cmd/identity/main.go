// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "identity",
		Short: "Identity management",
	}

	defaultConfDir = fpath.ApplicationDir("storj", "identity")
)

func main() {
	process.Exec(rootCmd)
}

func printExtensions(cert []byte, exts []pkix.Extension) error {
	hash, err := peertls.SHA256Hash(cert)
	if err != nil {
		return err
	}
	b64Hash, err := json.Marshal(hash)
	if err != nil {
		return err
	}
	fmt.Printf("Cert hash: %s\n", b64Hash)
	fmt.Println("Extensions:")
	for _, e := range exts {
		var data interface{}
		switch e.Id.String() {
		case peertls.ExtensionIDs[peertls.RevocationExtID].String():
			var rev peertls.Revocation
			if err := rev.Unmarshal(e.Value); err != nil {
				return err
			}
			data = rev
		default:
			data = e.Value
		}
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("\t%s: %s\n", e.Id, out)
	}
	return nil
}

func backupPath(path string) string {
	pathExt := filepath.Ext(path)
	base := strings.TrimSuffix(path, pathExt)
	return fmt.Sprintf(
		"%s.%s%s",
		base,
		strconv.Itoa(int(time.Now().Unix())),
		pathExt,
	)
}
