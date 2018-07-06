// Copyright (C) 2018 Storj Labs, Inc.
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

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/ranger"
)

var (
	addr           = flag.String("addr", "localhost:8080", "address to serve from")
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 20, "rs required")
	rsn            = flag.Int("total", 40, "rs total")
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
	encKey := sha256.Sum256([]byte(*key))
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	var firstNonce [12]byte
	decrypter, err := eestream.NewAESGCMDecrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}
	// initialize http rangers in parallel to save from network latency
	rrs := map[int]ranger.RangeCloser{}
	type indexRangerError struct {
		i   int
		rr  ranger.RangeCloser
		err error
	}
	result := make(chan indexRangerError, *rsn)
	for i := 0; i < *rsn; i++ {
		go func(i int) {
			url := fmt.Sprintf("http://18.184.133.99:%d", 10000+i)
			rr, err := ranger.HTTPRanger(url)
			result <- indexRangerError{i: i, rr: ranger.NopCloser(rr), err: err}
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
	rc, err := eestream.Decode(rrs, es, 4*1024*1024)
	if err != nil {
		return err
	}
	defer rc.Close()
	rr, err := eestream.Transform(rc, decrypter)
	if err != nil {
		return err
	}
	ctx := context.Background()
	rr, err = eestream.UnpadSlow(ctx, rr)
	if err != nil {
		return err
	}

	return http.ListenAndServe(*addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ranger.ServeContent(ctx, w, r, flag.Arg(0), time.Time{}, rr)
		}))
}
