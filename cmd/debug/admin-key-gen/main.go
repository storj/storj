// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jtolds/qod"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spacemonkeygo/flagfile"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/macaroon"
)

var (
	flagMacaroonHead   = flag.String("head", "", "hex-encoded admin macaroon head")
	flagMacaroonSecret = flag.String("secret", "", "hex-encoded admin macaroon secret")
)

func main() {
	flagfile.Load()
	err := run(context.Background())
	if err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	secret, err := hex.DecodeString(*flagMacaroonSecret)
	if err != nil {
		return err
	}
	if len(secret) != 32 {
		return errs.New("invalid secret length")
	}
	head, err := hex.DecodeString(*flagMacaroonHead)
	if err != nil {
		return err
	}
	if len(head) != 32 {
		return errs.New("invalid head length")
	}
	adminRoot, err := macaroon.NewUnrestrictedFixedHead(secret, head)
	if err != nil {
		return err
	}
	adminRootKey, err := macaroon.ParseRawAPIKey(adminRoot.Serialize())
	if err != nil {
		return err
	}

	for line := range qod.Lines(os.Stdin) {
		projectID, err := uuid.Parse(line)
		if err != nil {
			return err
		}

		expiry := time.Now().Add(time.Hour * 24)
		projectKey, err := adminRootKey.Restrict(macaroon.Caveat{
			ProjectId:       projectID[:],
			DisallowWrites:  true,
			DisallowDeletes: true,
			NotAfter:        &expiry,
		})
		if err != nil {
			return err
		}

		fmt.Println(projectKey.Serialize())
	}

	return nil
}
