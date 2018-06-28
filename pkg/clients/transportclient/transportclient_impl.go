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
func (o *transportClient) DialNode(ctx context.Context, addr string) (conn *grpc.ClientConn, err error) {
	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, addr)
}

// Dial using unauthenticated mode
func (o *transportClient) DialUnauthenticated(ctx context.Context, addr string) (conn *grpc.ClientConn, err error) {
	/* A dozen attempts... Recommendation: this value should be configurable */
	maxAttempts := 12
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
