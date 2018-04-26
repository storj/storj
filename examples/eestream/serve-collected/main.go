// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
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

func Main() error {
	encKey := sha256.Sum256([]byte(*key))
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	var firstNonce [24]byte
	decrypter, err := eestream.NewSecretboxDecrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
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
			rr, err := ranger.HTTPRanger(url)
			result <- indexRangerError{i, rr, err}
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
	rr, err := eestream.Decode(rrs, es)
	if err != nil {
		return err
	}
	rr, err = eestream.Transform(rr, decrypter)
	if err != nil {
		return err
	}
	rr, err = eestream.UnpadSlow(rr)
	if err != nil {
		return err
	}

	return http.ListenAndServe(*addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ranger.ServeContent(w, r, flag.Arg(0), time.Time{}, rr)
		}))
}
