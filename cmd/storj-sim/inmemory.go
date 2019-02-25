// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/processgroup"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pkcrypto"
)

func inmemoryRun(flags *Flags) error {
	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	planet, err := testplanet.NewWithLogger(log, flags.SatelliteCount, flags.StorageNodeCount, 0)
	if err != nil {
		return err
	}

	planet.Start(ctx)

	<-ctx.Done()
	err = ctx.Err()
	if err == context.Canceled {
		err = nil
	}

	return errs.Combine(err, planet.Shutdown(), log.Sync())
}

func inmemoryTest(flags *Flags, command string, args []string) error {
	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	planet, err := testplanet.NewWithLogger(log, flags.SatelliteCount, flags.StorageNodeCount, 0)
	if err != nil {
		return err
	}

	planet.Start(ctx)
	// wait a bit for kademlia to start
	time.Sleep(time.Second * 2)

	var env = os.Environ()

	// add bootstrap to environment
	bootstrap := planet.Bootstrap
	env = append(env, (&Info{
		Name:    "bootstrap/" + strconv.Itoa(0),
		ID:      bootstrap.ID().String(),
		Address: bootstrap.Addr(),
	}).Env()...)

	// add satellites to environment
	for i, satellite := range planet.Satellites {
		env = append(env, (&Info{
			Name:    "satellite/" + strconv.Itoa(i),
			ID:      satellite.ID().String(),
			Address: satellite.Addr(),
		}).Env()...)
	}

	// add storage nodes to environment
	for i, storage := range planet.StorageNodes {
		env = append(env, (&Info{
			Name:    "storage/" + strconv.Itoa(i),
			ID:      storage.ID().String(),
			Address: storage.Addr(),
		}).Env()...)
	}

	// add additional identities to the environment
	for i := 0; i < flags.Identities; i++ {
		identity, err := planet.NewIdentity()
		if err != nil {
			return errs.Combine(err, planet.Shutdown())
		}

		var chainPEM bytes.Buffer
		errLeaf := pkcrypto.WriteCertPEM(&chainPEM, identity.Leaf)
		errCA := pkcrypto.WriteCertPEM(&chainPEM, identity.CA)
		if errLeaf != nil || errCA != nil {
			return errs.Combine(errLeaf, errCA, planet.Shutdown())
		}

		var key bytes.Buffer
		errKey := pkcrypto.WritePrivateKeyPEM(&key, identity.Key)
		if errKey != nil {
			return errs.Combine(errKey, planet.Shutdown())
		}

		env = append(env,
			fmt.Sprintf("IDENTITY_%d_ID=%v", i, identity.ID.String()),
			fmt.Sprintf("IDENTITY_%d_KEY=%v", i, base64.StdEncoding.EncodeToString(key.Bytes())),
			fmt.Sprintf("IDENTITY_%d_CHAIN=%v", i, base64.StdEncoding.EncodeToString(chainPEM.Bytes())),
		)
	}

	// run the specified program
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	processgroup.Setup(cmd)

	errRun := cmd.Run()

	return errs.Combine(errRun, planet.Shutdown(), log.Sync())
}
