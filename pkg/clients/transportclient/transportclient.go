// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/connectivity"

	"google.golang.org/grpc"
)

// Dial using the authenticated mode
func DialNode(ctx context.Context, addr string) (conn *grpc.ClientConn, err error) {
	/* TODO@ASK security feature under development */
	return DialUnauthenticated(ctx, addr)
}

// Dial using unauthenticated mode
func DialUnauthenticated(ctx context.Context, addr string) (conn *grpc.ClientConn, err error) {
	if addr == "" {
		return nil, errors.New("node param uninitialized")
	} else {
		/* A dozen attempts... Recommendation: this value should be configurable */
		maxAttempts := 12
		/* err is nil, that means lookup passed complete info */
		conn, err = grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}

		/* connection retry attempt */
		for (conn.GetState() != connectivity.State(connectivity.Ready)) && ((maxAttempts <= 12) && (maxAttempts > 0)) {
			time.Sleep(15 * time.Millisecond)
			maxAttempts--
		}
		if maxAttempts <= 0 {
			return nil, errors.New("Connection failed to open using grpc")
		}
		return conn, err
	}
}
