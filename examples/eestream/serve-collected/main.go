// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"net/http"
	"os"
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
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Main is the exported CLI executable function
func Main() error {
	ctx := context.Background()
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
	// initialize http rangers in parallel to save from network latency
	rrs := map[int]ranger.Ranger{}
	type indexRangerError struct {
		i   int
		rr  ranger.Ranger
		err error
	}
	result := make(chan indexRangerError, *rsn)
	for i := 0; i < *rsn; i++ {
		go func(i int) {
			url := fmt.Sprintf("http://18.184.133.99:%d", 10000+i)
			rr, err := ranger.HTTPRanger(ctx, url)
			result <- indexRangerError{i: i, rr: rr, err: err}
		}(i)
	}
	// wait for all goroutines to finish and save result in rrs map
	for i := 0; i < *rsn; i++ {
		res := <-result
		if res.err != nil {
			// return on the first failure
			return err
		}
		rrs[res.i] = res.rr
	}
	rc, err := eestream.Decode(zap.L(), rrs, es, 4*1024*1024, false)
	if err != nil {
		return err
	}
	rr, err := encryption.Transform(rc, decrypter)
	if err != nil {
		return err
	}
	rr, err = encryption.UnpadSlow(ctx, rr)
	if err != nil {
		return err
	}

	return http.ListenAndServe(*addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ranger.ServeContent(ctx, w, r, flag.Arg(0), time.Time{}, rr)
		}))
}
