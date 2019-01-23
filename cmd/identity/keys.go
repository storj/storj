// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

var (
	keysCmd = &cobra.Command{
		Use:   "keys",
		Short: "Manage keys",
	}
	keyEncodeCmd = &cobra.Command{
		Use:   "encode <key path>[, ...]",
		Short: "encode binary key to PEM",
		RunE:  cmdKeyEncode,
		Args:  cobra.MinimumNArgs(1),
	}
	keyGenerateCmd = &cobra.Command{
		Use:         "generate",
		Short:       "generate lots of keys",
		RunE:        cmdKeyGenerate,
		Annotations: map[string]string{"type": "setup"},
	}

	keyCfg struct {
		MinDifficulty int    `help:"minimum difficulty to output" default:"30"`
		Concurrency   int    `help:"worker concurrency" default:"4"`
		OutputDir     string `help:"output directory to place keys" default:"."`
	}
)

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keyEncodeCmd)
	keysCmd.AddCommand(keyGenerateCmd)
	cfgstruct.Bind(keyEncodeCmd.Flags(), &keyCfg)
	cfgstruct.Bind(keyGenerateCmd.Flags(), &keyCfg)
}

// TODO: better errors?
func cmdKeyEncode(cmd *cobra.Command, args []string) error {
	keyErrs := new(errs.Group)
	for _, arg := range args {
		paths, err := filepath.Glob(arg)
		if err != nil {
			return err
		}
		for _, path := range paths {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				keyErrs.Add(err)
				continue
			}

			pemData := new(bytes.Buffer)
			if err = pem.Encode(pemData, peertls.NewKeyBlock(data)); err != nil {
				keyErrs.Add(err)
				continue
			}
			if err = peertls.WriteKeyData(path, pemData.Bytes()); err != nil {
				keyErrs.Add(err)
				continue
			}
		}
	}
	return keyErrs.Err()
}

func cmdKeyGenerate(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	err = os.MkdirAll(keyCfg.OutputDir, 0700)
	if err != nil {
		return err
	}
	counter := new(uint32)
	return identity.GenerateKeys(ctx, uint16(keyCfg.MinDifficulty), keyCfg.Concurrency,
		func(k *ecdsa.PrivateKey, id storj.NodeID) (done bool, err error) {
			data, err := x509.MarshalECPrivateKey(k)
			if err != nil {
				return false, err
			}
			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}
			filename := fmt.Sprintf("gen-%02d-%d.key", difficulty, atomic.AddUint32(counter, 1))
			fmt.Println("writing", filename)

			pemData := new(bytes.Buffer)
			if err = pem.Encode(pemData, peertls.NewKeyBlock(data)); err != nil {
				return false, err
			}
			if err = peertls.WriteKeyData(filepath.Join(keyCfg.OutputDir, filename), pemData.Bytes()); err != nil {
				return false, err
			}

			return false, err
		})
}
