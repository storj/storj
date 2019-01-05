// Copyright (c) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"crypto/ecdsa"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

// GenerateKey generates a private key with a node id with difficulty at least
// minDifficulty. No parallelism is used.
func GenerateKey(ctx context.Context, minDifficulty uint16) (
	k *ecdsa.PrivateKey, id storj.NodeID, err error) {
	var d uint16
	for {
		err = ctx.Err()
		if err != nil {
			break
		}
		k, err = peertls.NewKey()
		if err != nil {
			break
		}
		id, err = NodeIDFromECDSAKey(&k.PublicKey)
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

// GenerateKeys continues to generate keys until found returns done == false,
// or the ctx is canceled.
func GenerateKeys(ctx context.Context, minDifficulty uint16, concurrency int,
	found func(*ecdsa.PrivateKey, storj.NodeID) (done bool, err error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errchan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				k, id, err := GenerateKey(ctx, minDifficulty)
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

	return <-errchan
}
