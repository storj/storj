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
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"text/tabwriter"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/draw"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminalapi"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/prometheus/common/log"
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

	t, err := termbox.New()
	if err != nil {
		return err
	}
	defer t.Close()

	ctx, cancel := context.WithCancel(process.Ctx(cmd))
	quitter := func(k *terminalapi.Keyboard) {
		if k.Key == 'q' || k.Key == 'Q' {
			cancel()
		}
	}

	textWidget := text.New()

	c, err := container.New(t,
		container.Border(draw.LineStyleLight),
		container.BorderTitle(" batch identity generation | press \"q\" to quit "),
		container.PlaceWidget(textWidget),
	)

	//ctrl, err := termdash.NewController(t, c)
	//if err != nil {
	//	return err
	//}
	go func() {
		if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter)); err != nil {
			log.Fatal("termdash error: %s", err.Error())
		}
	}()

	if err := updateStatus(nil, textWidget); err != nil {
		return err
	}

	counter := new(uint32)
	diffCounts := make(map[uint16]*uint32)
	return identity.GenerateKeys(ctx, uint16(keyCfg.MinDifficulty), keyCfg.Concurrency,
		func(k *ecdsa.PrivateKey, id storj.NodeID) (done bool, err error) {
			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}

			if diffCounts[difficulty] == nil {
				diffCounts[difficulty] = new(uint32)
			}
			atomic.AddUint32(diffCounts[difficulty], 1)
			if err := updateStatus(diffCounts, textWidget); err != nil {
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

func updateStatus(counts map[uint16]*uint32, textWidget *text.Text) error {
	textBuf := new(bytes.Buffer)
	w := tabwriter.NewWriter(textBuf, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintf(w, "Difficulty\tCount\n"); err != nil {
		return err
	}
	for diff, count := range counts {
		if _, err := fmt.Fprintf(w, "%d\t%d\n", diff, atomic.LoadUint32(count)); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	textWidget.Reset()
	if err := textWidget.Write(textBuf.String()); err != nil {
		return err
	}
	return nil
}
