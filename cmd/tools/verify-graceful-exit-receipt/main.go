// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// verify-graceful-exit-receipt verifies the exit completion signature that is sent to
// a node on a failed or successful graceful exit.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/trust"
)

func main() {
	ctx := context.Background()

	isCompleted := flag.Bool("completed", false, "the hex input is a successful graceful exit")
	isFailed := flag.Bool("failed", false, "the hex input is a failed graceful exit")
	satelliteURL := flag.String("satellite", "", "where to fetch the satellite public signature")
	flag.Parse()

	hexinput := flag.Arg(0)

	data, err := hex.DecodeString(hexinput)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid hex input %q: %v\n", hexinput, err)
		os.Exit(1)
	}

	tryAll := !(*isCompleted || *isFailed)

	var errFailed, errCompleted error

	if *isFailed || tryAll {
		errFailed = handleExitFailed(ctx, *satelliteURL, data)
	}
	if *isCompleted || tryAll {
		errCompleted = handleExitCompleted(ctx, *satelliteURL, data)
	}

	err = errs.Combine(errFailed, errCompleted)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func handleExitFailed(ctx context.Context, satelliteurl string, data []byte) error {
	var exit pb.ExitFailed
	err := pb.Unmarshal(data, &exit)
	if err != nil {
		return fmt.Errorf("input does not seem to be %T message: %w", exit, err)
	}

	pretty, err := json.MarshalIndent(exit, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to encode %#v as json: %w", exit, err)
	}

	_, _ = fmt.Fprintln(os.Stdout, string(pretty))

	if satelliteurl != "" {
		signee, err := fetchSigneeFromSatellite(ctx, satelliteurl, exit.SatelliteId)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "UNABLE TO FETCH SIGNEE %q: %v\n", satelliteurl, err)
		} else {
			if err := signing.VerifyExitFailed(ctx, signee, &exit); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "SIGNATURE INVALID: %v\n", err)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "SIGNATURE VALID\n")
			}
		}
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "UNABLE TO VERIFY SIGNATURE, SIGNEE MISSING\n")
	}

	return nil
}

func handleExitCompleted(ctx context.Context, satelliteurl string, data []byte) error {
	var exit pb.ExitCompleted
	err := pb.Unmarshal(data, &exit)
	if err != nil {
		return fmt.Errorf("input does not seem to be %T message: %w", exit, err)
	}

	pretty, err := json.MarshalIndent(exit, "", "    ")
	if err != nil {
		return fmt.Errorf("unable to encode %#v as json: %w", exit, err)
	}

	_, _ = fmt.Fprintln(os.Stdout, string(pretty))

	if satelliteurl != "" {
		signee, err := fetchSigneeFromSatellite(ctx, satelliteurl, exit.SatelliteId)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "UNABLE TO FETCH SIGNEE %q: %v\n", satelliteurl, err)
		} else {
			if err := signing.VerifyExitCompleted(ctx, signee, &exit); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "SIGNATURE INVALID: %v\n", err)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "SIGNATURE VALID\n")
			}
		}
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "UNABLE TO VERIFY SIGNATURE, SIGNEE MISSING\n")
	}
	return nil
}

func fetchSigneeFromSatellite(ctx context.Context, satelliteurl string, id storj.NodeID) (signing.Signee, error) {
	satellite, err := storj.ParseNodeURL(satelliteurl)
	if err != nil {
		return nil, fmt.Errorf("does not seem to be a node url: %w", err)
	}

	if satellite.ID.IsZero() {
		satellite.ID = id
	}

	tlsOptions, err := minimalTLSOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS dialing options: %w", err)
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)
	trustDialer := trust.Dialer(dialer)

	identity, err := trustDialer.ResolveIdentity(ctx, satellite)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve identity: %w", err)
	}

	return signing.SigneeFromPeerIdentity(identity), nil
}

func minimalTLSOptions(ctx context.Context) (*tlsopts.Options, error) {
	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		return nil, err
	}

	config := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	opts, err := tlsopts.NewOptions(ident, config, nil)
	if err != nil {
		return nil, err
	}

	return opts, nil
}
