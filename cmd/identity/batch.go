// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/tar"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
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

type deferredIdentity struct {
	key   *ecdsa.PrivateKey
	id    storj.NodeID
	files map[string][]byte
	path  string
}

var (
	keyGenerateCmd = &cobra.Command{
		Use:         "batch-generate",
		Short:       "generate lots of keys",
		RunE:        cmdKeyGenerate,
		Annotations: map[string]string{"type": "setup"},
	}

	keyCfg struct {
		MinDifficulty int    `help:"minimum difficulty to output" default:"30"`
		Concurrency   int    `help:"worker concurrency" default:"4"`
		OutputDir     string `help:"output directory to place keys" default:"."`
		Permissions   uint   `default:"0700" help:"permissions to apply to output directories and/or files"`
		Tar           bool   ` default:"false" help:"f true, identities will be output as tarballs instead of directories"`
	}
)

func init() {
	rootCmd.AddCommand(keyGenerateCmd)
	cfgstruct.Bind(keyGenerateCmd.Flags(), &keyCfg)
}

func cmdKeyGenerate(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	err = os.MkdirAll(keyCfg.OutputDir, os.FileMode(keyCfg.Permissions))
	if err != nil {
		return err
	}
	counter := new(uint32)
	return identity.GenerateKeys(ctx, uint16(keyCfg.MinDifficulty), keyCfg.Concurrency,
		func(k *ecdsa.PrivateKey, id storj.NodeID) (done bool, err error) {
			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}

			genName := fmt.Sprintf("gen-%02d-%d", difficulty, atomic.AddUint32(counter, 1))
			dID := deferredIdentity{
				path: filepath.Join(keyCfg.OutputDir, genName),
				key:  k,
				id:   id,
			}

			if err := dID.build(); err != nil {
				return false, err
			}

			idPathPart := id.String()[:10]
			if keyCfg.Tar {
				if err := dID.writeTar(idPathPart); err != nil {
					return false, err
				}
			} else {
				if err := dID.writeDir(idPathPart); err != nil {
					return false, err
				}
			}
			return false, err
		})
}

func (d *deferredIdentity) build() error {
	d.files = make(map[string][]byte)

	ct, err := peertls.CATemplate()
	if err != nil {
		return errs.Wrap(err)
	}

	caCert, err := peertls.NewCert(d.key, nil, ct, nil)
	if err != nil {
		return errs.Wrap(err)
	}

	ca := &identity.FullCertificateAuthority{
		Cert: caCert,
		ID:   d.id,
		Key:  d.key,
	}

	ident, err := ca.NewIdentity()
	if err != nil {
		return errs.Wrap(err)
	}

	caCertBytes := new(bytes.Buffer)
	if err := peertls.WriteChain(caCertBytes, ca.Cert); err != nil {
		return errs.Wrap(err)
	}

	identBytes := new(bytes.Buffer)
	if err := peertls.WriteChain(identBytes, ident.Leaf, ident.CA); err != nil {
		return errs.Wrap(err)
	}

	for name, chain := range map[string][]*x509.Certificate{
		"ca.cert":       {ca.Cert},
		"identity.cert": {ident.Leaf, ident.CA},
	} {
		chainBuf := new(bytes.Buffer)
		if err := peertls.WriteChain(chainBuf, chain...); err != nil {
			return errs.Wrap(err)
		}
		d.files[name] = chainBuf.Bytes()
	}

	for name, key := range map[string]crypto.PrivateKey{
		"ca.key":       ca.Key,
		"identity.key": ident.Key,
	} {
		keyBuf := new(bytes.Buffer)
		if err := peertls.WriteKey(keyBuf, key); err != nil {
			return errs.Wrap(err)
		}
		d.files[name] = keyBuf.Bytes()
	}

	return nil
}

func (d *deferredIdentity) writeDir(idPathPart string) error {
	idDir := filepath.Join(d.path, idPathPart)
	if err := os.MkdirAll(idDir, os.FileMode(keyCfg.Permissions)); err != nil {
		return errs.Wrap(err)
	}

	for name, data := range d.files {
		if err := ioutil.WriteFile(filepath.Join(idDir, name), data, os.FileMode(keyCfg.Permissions)); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

func (d *deferredIdentity) writeTar(idPathPart string) error {
	tarData := new(bytes.Buffer)
	tw := tar.NewWriter(tarData)

	for name, data := range d.files {
		err := writeToTar(tw, filepath.Join(idPathPart, name), data)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	if err := tw.Close(); err != nil {
		return errs.Wrap(err)
	}

	err := ioutil.WriteFile(d.path+".tar", tarData.Bytes(), os.FileMode(keyCfg.Permissions))
	if err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func writeToTar(tw *tar.Writer, name string, data []byte) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: int64(keyCfg.Permissions),
		Size: int64(len(data)),
	})
	if err != nil {
		return err
	}

	_, err = tw.Write(data)
	if err != nil {
		return err
	}
	return nil
}
