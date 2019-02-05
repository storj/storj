// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/cui"
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
	ctx, cancel := context.WithCancel(process.Ctx(cmd))
	defer cancel()

	err = os.MkdirAll(keyCfg.OutputDir, os.FileMode(keyCfg.Permissions))
	if err != nil {
		return err
	}

	var group errgroup.Group
	defer func() {
		err = errs.Combine(err, group.Wait())
	}()

	screen, err := cui.NewScreen()
	if err != nil {
		return err
	}
	group.Go(func() error {
		defer cancel()
		err := screen.Run()
		return errs.Combine(err, screen.Close())
	})

	counter := new(uint32)
	diffCounts := [256]uint32{}
	return identity.GenerateKeys(ctx, uint16(keyCfg.MinDifficulty), keyCfg.Concurrency,
		func(k *ecdsa.PrivateKey, id storj.NodeID) (done bool, err error) {
			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}

			if int(difficulty) > len(diffCounts) {
				atomic.AddUint32(&diffCounts[len(diffCounts)-1], 1)
			} else {
				atomic.AddUint32(&diffCounts[difficulty], 1)
			}

			if err := renderStats(screen, diffCounts[:]); err != nil {
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

var renderingMutex sync.Mutex

func renderStats(screen *cui.Screen, stats []uint32) error {
	renderingMutex.Lock()
	defer renderingMutex.Unlock()

	var err error
	printf := func(w io.Writer, format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, format, args...)
	}

	printf(screen, "\n")
	printf(screen, "batch identity creation\n")
	printf(screen, "=======================\n\n")

	w := tabwriter.NewWriter(screen, 0, 2, 2, ' ', 0)
	printf(w, "Difficulty\tCount\n")

	for difficulty := 0; difficulty < len(stats); difficulty++ {
		count := atomic.LoadUint32(&stats[difficulty])
		if count == 0 {
			continue
		}
		printf(w, "%d\t%d\n", difficulty, count)
	}

	err = errs.Combine(err, w.Flush())

	return screen.Flush()
}
