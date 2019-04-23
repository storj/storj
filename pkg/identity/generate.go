// Copyright (c) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"crypto"

	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

// GenerateKey generates a private key with a node id with difficulty at least
// minDifficulty. No parallelism is used.
func GenerateKey(ctx context.Context, minDifficulty uint16, version storj.IDVersion) (
	k crypto.PrivateKey, id storj.NodeID, err error) {
	var d uint16
	for {
		err = ctx.Err()
		if err != nil {
			break
		}
		k, err = pkcrypto.GeneratePrivateKey()
		if err != nil {
			break
		}
		id, err = NodeIDFromKey(pkcrypto.PublicKeyFromPrivate(k), version)
		if err != nil {
			break
		}
		d, err = id.Difficulty()
		if err != nil {
			break
		}
		if d >= minDifficulty {
			return k, id, nil
		}
	}
	return k, id, storj.ErrNodeID.Wrap(err)
}

// GenerateCallback indicates that key generation is done when done is true.
// if err != nil key generation will stop with that error
type GenerateCallback func(crypto.PrivateKey, storj.NodeID) (done bool, err error)

// GenerateKeyWithCounterCallback indicates that key generation is done when done is true.
// if err != nil key generation will stop with that error
type GenerateKeyWithCounterCallback func(crypto.PrivateKey, peertls.POWCounter, storj.NodeID) (done bool, err error)

// GenerateKeys continues to generate keys until found returns done == true,
// or the ctx is canceled.
func GenerateKeys(ctx context.Context, minDifficulty uint16, concurrency int, version storj.IDVersion, found GenerateCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	group, _ := errgroup.WithContext(ctx)

	for i := 0; i < concurrency; i++ {
		group.Go(func() error {
			defer cancel()
			for {
				if ctxDone(ctx) {
					return nil
				}

				k, id, err := GenerateKey(ctx, minDifficulty, version)
				if err != nil {
					return err
				}

				done, err := found(k, id)
				if err != nil {
					return err
				}
				if done {
					return err
				}
			}
		})
	}

	return group.Wait()
}

// GenerateKeyWithCounter generates a key and continues to increment the counter
// until found returns done == true or the ctx is canceled.
func GenerateKeyKeyWithCounter(ctx context.Context, minDifficulty uint16, concurrency int, version storj.IDVersion, found GenerateKeyWithCounterCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	group, _ := errgroup.WithContext(ctx)

	k, _, err := GenerateKey(ctx, minDifficulty, version)
	if err != nil {
		return err
	}

	count := atomic.NewUint64(0)
	for i := 0; i < concurrency; i++ {
		group.Go(func() error {
			defer cancel()
			for {
				if ctxDone(ctx) {
					return nil
				}

				counter := peertls.POWCounter(count.Inc())
				//time.Sleep(25 * time.Millisecond)
				id, err := NodeIDFromKeyWithCounter(pkcrypto.PublicKeyFromPrivate(k), counter, version)
				if err != nil {
					return err
				}

				done, err := found(k, counter, id)
				if err != nil {
					return err
				}
				if done {
					return err
				}
			}
		})
	}

	return group.Wait()
}

func ctxDone(ctx context.Context) bool {
	ctxDone := ctx.Done()
	select {
	case <-ctxDone:
		return true
	default:
		return false
	}
}
