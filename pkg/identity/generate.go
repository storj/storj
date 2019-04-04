// Copyright (c) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"crypto"

	"go.uber.org/atomic"

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
	defer cancel()
	errchan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				k, id, err := GenerateKey(ctx, minDifficulty, version)
				if err != nil {
					errchan <- err
					return
				}

				done, err := found(k, id)
				if err != nil {
					errchan <- err
					return
				}
				if done {
					errchan <- nil
					return
				}
			}
		}()
	}

	// we only care about the first error. the rest of the errors will be
	// context cancellation errors
	return <-errchan
}

// GenerateKeyWithCounter generates a key and continues to increment the counter
// until found returns done == true or the ctx is canceled.
func GenerateKeyKeyWithCounter(ctx context.Context, minDifficulty uint16, concurrency int, version storj.IDVersion, found GenerateKeyWithCounterCallback) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errchan := make(chan error, concurrency)

	k, id, err := GenerateKey(ctx, minDifficulty, version)
	if err != nil {
		return err
	}

	count := atomic.NewUint64(0)
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			for {
				counter := peertls.POWCounter(count.Inc())
				id, err = NodeIDFromKeyWithCounter(pkcrypto.PublicKeyFromPrivate(k), counter, version)
				if err != nil {
					errchan <- err
					return
				}

				done, err := found(k, counter, id)
				if err != nil {
					errchan <- err
					return
				}
				if done {
					errchan <- nil
					return
				}
			}
		}(i)
	}

	// we only care about the first error. the rest of the errors will be
	// context cancellation errors
	return <-errchan
}