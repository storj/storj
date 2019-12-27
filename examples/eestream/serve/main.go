// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vivint/infectious"
	"go.uber.org/zap"

	"storj.io/common/encryption"
	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/storj/uplink/eestream"
)

var (
	addr             = flag.String("addr", "localhost:8080", "address to serve from")
	erasureShareSize = flag.Int("erasure_share_size", 4*1024, "block size of pieces")
	key              = flag.String("key", "a key", "the secret key")
	rsk              = flag.Int("required", 20, "rs required")
	rsn              = flag.Int("total", 40, "rs total")
)

func main() {
	flag.Parse()
	if flag.Arg(0) == "" {
		fmt.Printf("usage: %s <targetdir>\n", os.Args[0])
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
	encKey := storj.Key(sha256.Sum256([]byte(*key)))
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *erasureShareSize)
	var firstNonce storj.Nonce
	decrypter, err := encryption.NewDecrypter(storj.EncAESGCM, &encKey, &firstNonce, es.StripeSize())
	if err != nil {
		return err
	}
	pieces, err := ioutil.ReadDir(flag.Arg(0))
	if err != nil {
		return err
	}
	rrs := map[int]ranger.Ranger{}
	for _, piece := range pieces {
		piecenum, err := strconv.Atoi(strings.TrimSuffix(piece.Name(), ".piece"))
		if err != nil {
			return err
		}
		r, err := ranger.FileRanger(filepath.Join(flag.Arg(0), piece.Name()))
		if err != nil {
			return err
		}
		rrs[piecenum] = r
	}
	rc, err := eestream.Decode(zap.L(), rrs, es, 4*1024*1024, false)
	if err != nil {
		return err
	}
	rr, err := encryption.Transform(rc, decrypter)
	if err != nil {
		return err
	}
	ctx := context.Background()
	rr, err = encryption.UnpadSlow(ctx, rr)
	if err != nil {
		return err
	}

	return http.ListenAndServe(*addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ranger.ServeContent(ctx, w, r, flag.Arg(0), time.Time{}, rr)
		}))
}
