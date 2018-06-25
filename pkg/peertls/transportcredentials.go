package peertls

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"google.golang.org/grpc/credentials"
)

// type TransportCredentials interface {
//     ClientHandshake(context.Context, string, net.Conn) (net.Conn, AuthInfo, error)
//     ServerHandshake(net.Conn) (net.Conn, AuthInfo, error)
//     Info() ProtocolInfo
//     Clone() TransportCredentials
//     OverrideServerName(string) error
// }

type tlsCredsWrapper struct {
	tlsCreds    *credentials.TransportCredentials
	config      *tls.Config
	configMutex *sync.Mutex
}

func (t *tlsCredsWrapper) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	t.configMutex.Lock()
	defer t.configMutex.Unlock()

	// use local cfg to avoid clobbering ServerName if using multiple endpoints
	// cfg := cloneTLSConfig(t.config)
	// if cfg.ServerName == "" {
	// 	colonPos := strings.LastIndex(authority, ":")
	// 	if colonPos == -1 {
	// 		colonPos = len(authority)
	// 	}
	// 	cfg.ServerName = authority[:colonPos]
	// }
	conn := tls.Client(rawConn, t.config)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- conn.Handshake()
	}()
	select {
	case err := <-errChannel:
		if err != nil {
			return nil, nil, err
		}
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	return conn, credentials.TLSInfo{conn.ConnectionState()}, nil
}

func (t *tlsCredsWrapper) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	t.configMutex.Lock()
	defer t.configMutex.Unlock()

	conn := tls.Server(rawConn, t.config)
	if err := conn.Handshake(); err != nil {
		return nil, nil, err
	}
	return conn, credentials.TLSInfo{conn.ConnectionState()}, nil
}

func (t *tlsCredsWrapper) Info() credentials.ProtocolInfo {
	return t.Info()
}

func (t *tlsCredsWrapper) Clone() credentials.TransportCredentials {
	return t.Clone()
}

func (t *tlsCredsWrapper) OverrideServerName(serverNameOverride string) error {
	return t.OverrideServerName(serverNameOverride)
}

// * Copyright 2017 gRPC authors.
// * Licensed under the Apache License, Version 2.0 (the "License");
// * (see https://github.com/grpc/grpc-go/blob/v1.13.0/credentials/credentials_util_go18.go)
// cloneTLSConfig returns a shallow clone of the exported
// fields of cfg, ignoring the unexported sync.Once, which
// contains a mutex and must not be copied.
//
// If cfg is nil, a new zero tls.Config is returned.
func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}

	return cfg.Clone()
}
