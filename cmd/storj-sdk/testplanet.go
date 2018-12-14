// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/processgroup"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

func runTestPlanet(flags *Flags, command string, args []string) error {
	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	planet, err := testplanet.NewWithLogger(zap.L(), flags.SatelliteCount, flags.StorageNodeCount, 0)
	if err != nil {
		return err
	}

	planet.Start(ctx)
	// wait a bit for kademlia to start
	time.Sleep(time.Second * 2)

	var env = os.Environ()

	// add satellites to environment
	for i, satellite := range planet.Satellites {
		env = append(env,
			fmt.Sprintf("SATELLITE%d_ID=%v", i, satellite.ID().String()),
			fmt.Sprintf("SATELLITE%d_ADDR=%v", i, satellite.Addr()),
		)
	}

	// add storage nodes to environment
	for i, storage := range planet.StorageNodes {
		env = append(env,
			fmt.Sprintf("STORAGE%d_ID=%v", i, storage.ID().String()),
			fmt.Sprintf("STORAGE%d_ADDR=%v", i, storage.Addr()),
		)
	}

	// add additional identities to the environment
	for i := 0; i < flags.Identities; i++ {
		identity, err := planet.NewIdentity()
		if err != nil {
			return utils.CombineErrors(err, planet.Shutdown())
		}

		var chainPEM bytes.Buffer
		errLeaf := pem.Encode(&chainPEM, peertls.NewCertBlock(identity.Leaf.Raw))
		errCA := pem.Encode(&chainPEM, peertls.NewCertBlock(identity.CA.Raw))
		if errLeaf != nil || errCA != nil {
			return utils.CombineErrors(errLeaf, errCA, planet.Shutdown())
		}

		var key bytes.Buffer
		errKey := peertls.WriteKey(&key, identity.Key)
		if errKey != nil {
			return utils.CombineErrors(errKey, planet.Shutdown())
		}

		env = append(env,
			fmt.Sprintf("IDENTITY%d_ID=%v", i, identity.ID.String()),
			fmt.Sprintf("IDENTITY%d_KEY=%v", i, base64.StdEncoding.EncodeToString(key.Bytes())),
			fmt.Sprintf("IDENTITY%d_CHAIN=%v", i, base64.StdEncoding.EncodeToString(chainPEM.Bytes())),
		)
	}

	// run the specified program
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	processgroup.Setup(cmd)

	errRun := cmd.Run()

	return utils.CombineErrors(errRun, planet.Shutdown())
}
