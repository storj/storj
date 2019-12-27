// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/tar"
	"bytes"
	"crypto"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/common/peertls"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/private/cui"
)

var (
	certMode       int64 = 0644
	keyMode        int64 = 0600
	keyGenerateCmd       = &cobra.Command{
		Use:         "batch-generate",
		Short:       "generate lots of keys",
		RunE:        cmdKeyGenerate,
		Annotations: map[string]string{"type": "setup"},
	}

	keyCfg struct {
		// TODO: where is this used and should it be conistent with "latest" alias?
		VersionNumber uint   `default:"0" help:"version of identity (0 is latest)"`
		MinDifficulty int    `help:"minimum difficulty to output" default:"36"`
		Concurrency   int    `help:"worker concurrency" default:"4"`
		OutputDir     string `help:"output directory to place keys" default:"."`
	}
	defaults cfgstruct.BindOpt
)

func init() {
	defaults = cfgstruct.DefaultsFlag(rootCmd)
	rootCmd.AddCommand(keyGenerateCmd)
	process.Bind(keyGenerateCmd, &keyCfg, defaults)
}

func cmdKeyGenerate(cmd *cobra.Command, args []string) (err error) {
	ctx, cancel := process.Ctx(cmd)
	defer cancel()

	err = os.MkdirAll(keyCfg.OutputDir, 0700)
	if err != nil {
		return err
	}

	version, err := storj.GetIDVersion(storj.IDVersionNumber(keyCfg.VersionNumber))
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
	if err := renderStats(screen, diffCounts[:]); err != nil {
		return err
	}
	return identity.GenerateKeys(ctx, uint16(keyCfg.MinDifficulty), keyCfg.Concurrency, version,
		func(k crypto.PrivateKey, id storj.NodeID) (done bool, err error) {
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
			err = saveIdentityTar(filepath.Join(keyCfg.OutputDir, genName), k, id)
			return false, err
		})
}

func saveIdentityTar(path string, key crypto.PrivateKey, id storj.NodeID) error {
	ct, err := peertls.CATemplate()
	if err != nil {
		return err
	}

	caCert, err := peertls.CreateSelfSignedCertificate(key, ct)
	if err != nil {
		return err
	}

	ca := &identity.FullCertificateAuthority{
		Cert: caCert,
		ID:   id,
		Key:  key,
	}

	ident, err := ca.NewIdentity()
	if err != nil {
		return err
	}

	tarData := new(bytes.Buffer)
	tw := tar.NewWriter(tarData)

	caCertBytes, caCertErr := peertls.ChainBytes(ca.Cert)
	caKeyBytes, caKeyErr := pkcrypto.PrivateKeyToPEM(ca.Key)
	identCertBytes, identCertErr := peertls.ChainBytes(ident.Leaf, ident.CA)
	identKeyBytes, identKeyErr := pkcrypto.PrivateKeyToPEM(ident.Key)
	if err := errs.Combine(caCertErr, caKeyErr, identCertErr, identKeyErr); err != nil {
		return err
	}

	if err := errs.Combine(
		writeToTar(tw, "ca.cert", certMode, caCertBytes),
		writeToTar(tw, "ca.key", keyMode, caKeyBytes),
		writeToTar(tw, "identity.cert", certMode, identCertBytes),
		writeToTar(tw, "identity.key", keyMode, identKeyBytes),
	); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return errs.Wrap(err)
	}

	if err = ioutil.WriteFile(path+".tar", tarData.Bytes(), 0600); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func writeToTar(tw *tar.Writer, name string, mode int64, data []byte) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: mode,
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

func renderStats(screen *cui.Screen, stats []uint32) error {
	screen.Lock()
	defer screen.Unlock()

	var err error
	printf := func(w io.Writer, format string, args ...interface{}) {
		if err == nil {
			_, err = fmt.Fprintf(w, format, args...)
		}
	}

	printf(screen, "Batch Identity Creation\n\n\n")

	w := tabwriter.NewWriter(screen, 0, 2, 2, ' ', 0)
	printf(w, "Difficulty\tCount\n")

	total := uint32(0)

	for difficulty := len(stats) - 1; difficulty >= 0; difficulty-- {
		count := atomic.LoadUint32(&stats[difficulty])
		total += count
		if count == 0 {
			continue
		}
		printf(w, "%d\t%d\n", difficulty, count)
	}
	printf(w, "Total\t%d\n", total)

	err = errs.Combine(err, w.Flush())

	return screen.Flush()
}
