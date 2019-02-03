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
	"log"
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
	key *ecdsa.PrivateKey
	id  storj.NodeID

	ca    *identity.FullCertificateAuthority
	ident *identity.FullIdentity

	tarPath string
	tarData *bytes.Buffer

	errs *errs.Group
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
	}
)

func init() {
	rootCmd.AddCommand(keyGenerateCmd)
	cfgstruct.Bind(keyGenerateCmd.Flags(), &keyCfg)
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
			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}

			genName := fmt.Sprintf("gen-%02d-%d.tar", difficulty, atomic.AddUint32(counter, 1))
			dID := deferredIdentity{
				tarPath: filepath.Join(keyCfg.OutputDir, genName),
				key:     k,
				id:      id,

				tarData: new(bytes.Buffer),

				errs: new(errs.Group),
			}

			go dID.build().writeTar(keyCfg.OutputDir).done()

			return false, err
		})
}

func (d *deferredIdentity) build() *deferredIdentity {
	ct, err := peertls.CATemplate()
	if err != nil {
		d.errs.Add(errs.New("CA template error: %s\n", err.Error()))
		return d
	}

	caCert, err := peertls.NewCert(d.key, nil, ct, nil)
	if err != nil {
		d.errs.Add(errs.New("new cert error: %s\n", err.Error()))
		return d
	}

	d.ca = &identity.FullCertificateAuthority{
		Cert: caCert,
		ID:   d.id,
		Key:  d.key,
	}

	d.ident, err = d.ca.NewIdentity()
	if err != nil {
		d.errs.Add(errs.New("new identity from CA error: %s\n", err.Error()))
		return d
	}
	return d
}

func (d *deferredIdentity) writeTar(outputDir string) *deferredIdentity {
	if err := d.errs.Err(); err != nil {
		return d
	}

	caCertBytes := new(bytes.Buffer)
	if err := peertls.WriteChain(caCertBytes, d.ca.Cert); err != nil {
		d.errs.Add(errs.New("error writing CA chain: %s\n", err.Error()))
		return d
	}

	identBytes := new(bytes.Buffer)
	if err := peertls.WriteChain(identBytes, d.ident.Leaf, d.ident.CA); err != nil {
		d.errs.Add(errs.New("error writing identity chain: %s\n", err.Error()))
		return d
	}

	idPathPart := d.id.String()[:10]

	tw := tar.NewWriter(d.tarData)

	for name, chain := range map[string][]*x509.Certificate{
		"ca.cert":       {d.ca.Cert},
		"identity.cert": {d.ident.Leaf, d.ident.CA},
	} {
		chainBuf := new(bytes.Buffer)
		if err := peertls.WriteChain(chainBuf, chain...); err != nil {
			d.errs.Add(errs.New("error writing identity chain: %s\n", err.Error()))
			return d
		}
		err := writeToTar(tw, filepath.Join(idPathPart, name), chainBuf.Bytes())
		if err != nil {
			d.errs.Add(errs.New("error writing to tar: %s\n", err.Error()))
			return d
		}
	}

	for name, key := range map[string]crypto.PrivateKey{
		"ca.key":       d.ca.Key,
		"identity.key": d.ident.Key,
	} {
		keyBuf := new(bytes.Buffer)
		if err := peertls.WriteKey(keyBuf, key); err != nil {
			d.errs.Add(errs.New("error writing key: %s\n", err.Error()))
			return d
		}
		err := writeToTar(tw, filepath.Join(idPathPart, name), keyBuf.Bytes())
		if err != nil {
			d.errs.Add(errs.New("error writing to tar: %s\n", err.Error()))
			return d
		}
	}

	if err := tw.Close(); err != nil {
		d.errs.Add(errs.New("error closing tar: %s\n", err.Error()))
		return d
	}

	err := ioutil.WriteFile(filepath.Join(d.tarPath), d.tarData.Bytes(), 0600)
	if err != nil {
		d.errs.Add(errs.New("error writing tar to disk: %s\n", err.Error()))
		return d
	}
	return d
}

func (d *deferredIdentity) done() error {
	if err := d.errs.Err(); err != nil {
		log.Fatal(err.Error())
		return err
	}
	return nil
}

func writeToTar(tw *tar.Writer, name string, data []byte) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0600,
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
