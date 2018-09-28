// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/eestream"
)

var (
	erasureShareSize = flag.Int("erasure_share_size", 4*1024, "block size of pieces")
	key              = flag.String("key", "a key", "the secret key")
	rsk              = flag.Int("required", 20, "rs required")
	rsn              = flag.Int("total", 40, "rs total")
)

func main() {
	flag.Parse()
	if flag.Arg(0) == "" {
		fmt.Printf("usage: cat data | %s <targetdir>\n", os.Args[0])
		os.Exit(1)
	}
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Main is the exported CLI executable function
func Main() error {
	err := os.MkdirAll(flag.Arg(0), 0755)
	if err != nil {
		return err
	}
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *erasureShareSize)
	rs, err := eestream.NewRedundancyStrategy(es, 0, 0)
	if err != nil {
		return err
	}
	encKey := eestream.Key(sha256.Sum256([]byte(*key)))
	var firstNonce eestream.Nonce
	cipher := eestream.AESGCM
	encrypter, err := cipher.NewEncrypter(&encKey, &firstNonce, es.StripeSize())
	if err != nil {
		return err
	}
	readers, err := eestream.EncodeReader(context.Background(),
		eestream.TransformReader(eestream.PadReader(os.Stdin,
			encrypter.InBlockSize()), encrypter, 0), rs, 4*1024*1024)
	if err != nil {
		return err
	}
	errs := make(chan error, len(readers))
	for i := range readers {
		go func(i int) {
			pieceFile := filepath.Join(flag.Arg(0), fmt.Sprintf("%d.piece", i))
			fh, err := os.Create(pieceFile)
			if err != nil {
				errs <- err
				return
			}

			defer printError(fh.Close)

			_, err = io.Copy(fh, readers[i])
			errs <- err
		}(i)
	}
	for range readers {
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
