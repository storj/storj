// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

func newV0CA(ctx context.Context, opts NewCAOptions) (_ *FullCertificateAuthority, err error) {
	// NB: `i` and `highscore` are only used for logging.
	var (
		highscore    = new(uint32)
		i            = new(uint32)
		updateStatus = statusLogger(opts.Logger, i, highscore)

		mu          sync.Mutex
		selectedKey crypto.PrivateKey
		selectedID  storj.NodeID
		extraExtensions = []pkix.Extension{NewVersionExt(storj.IDVersions[storj.V0])}
	)

	err = GenerateKeys(ctx, minimumLoggableDifficulty, int(opts.Concurrency), storj.IDVersions[storj.V0],
		func(k crypto.PrivateKey, id storj.NodeID) (done bool, err error) {
			if opts.Logger != nil {
				if atomic.AddUint32(i, 1)%100 == 0 {
					updateStatus()
				}
			}

			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}
			if difficulty >= opts.Difficulty {
				mu.Lock()
				if selectedKey == nil {
					updateStatus()
					selectedKey = k
					selectedID = id
				}
				mu.Unlock()
				if opts.Logger != nil {
					atomic.SwapUint32(highscore, uint32(difficulty))
					updateStatus()
					_, err := fmt.Fprintf(opts.Logger, "\nFound a key with difficulty %d!\n", difficulty)
					if err != nil {
						log.Print(errs.Wrap(err))
					}
				}
				return true, nil
			}
			for {
				hs := atomic.LoadUint32(highscore)
				if uint32(difficulty) <= hs {
					return false, nil
				}
				if atomic.CompareAndSwapUint32(highscore, hs, uint32(difficulty)) {
					updateStatus()
					return false, nil
				}
			}
		})
	if err != nil {
		return nil, err
	}
	return buildCA(opts, selectedKey, selectedID, extraExtensions)
}

func newV1CA(ctx context.Context, opts NewCAOptions) (_ *FullCertificateAuthority, err error) {
	// NB: `i` and `highscore` are only used for logging.
	var (
		highscore    = new(uint32)
		i            = new(uint32)
		updateStatus = statusLogger(opts.Logger, i, highscore)

		mu              sync.Mutex
		selectedKey     crypto.PrivateKey
		selectedID      storj.NodeID
		selectedCount   peertls.POWCounter
		extraExtensions = []pkix.Extension{NewVersionExt(storj.IDVersions[storj.V1])}
	)

	err = GenerateKeyKeyWithCounter(ctx, minimumLoggableDifficulty, int(opts.Concurrency), storj.IDVersions[storj.V1],
		func(k crypto.PrivateKey, counter peertls.POWCounter, id storj.NodeID) (done bool, err error) {
			select {
			case <-ctx.Done():
				return false, nil
			default:
				break
			}

			if opts.Logger != nil {
				if atomic.AddUint32(i, 1)%100 == 0 {
					updateStatus()
				}
			}

			difficulty, err := id.Difficulty()
			if err != nil {
				return false, err
			}
			if difficulty >= opts.Difficulty {
				mu.Lock()
				if selectedKey == nil {
					updateStatus()
					selectedKey = k
					selectedID = id
					selectedCount = counter
				}
				mu.Unlock()
				if opts.Logger != nil {
					atomic.SwapUint32(highscore, uint32(difficulty))
					updateStatus()
					_, err := fmt.Fprintf(opts.Logger, "\nFound a key with difficulty %d!\n", difficulty)
					if err != nil {
						log.Print(errs.Wrap(err))
					}
				}
				return true, nil
			}
			for {
				hs := atomic.LoadUint32(highscore)
				if uint32(difficulty) <= hs {
					return false, nil
				}
				if atomic.CompareAndSwapUint32(highscore, hs, uint32(difficulty)) {
					updateStatus()
					return false, nil
				}
			}
		})
	if err != nil {
		return nil, err
	}
	extraExtensions = append(extraExtensions, NewPOWCounterExt(selectedCount))
	return buildCA(opts, selectedKey, selectedID, extraExtensions)
}

