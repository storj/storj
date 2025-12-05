// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package testredis is package for starting a redis test server
package testredis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"storj.io/storj/shared/processgroup"
)

const (
	fallbackAddr = "localhost:6379"
	fallbackPort = 6379
)

// Server represents a redis server.
type Server interface {
	Addr() string
	Close() error
	// FastForward is a function for enforce the TTL of keys in
	// implementations what they have not exercise the expiration by themselves
	// (e.g. Minitredis). This method is a no-op in implementations which support
	// the expiration as usual.
	//
	// All the keys whose TTL minus d become <= 0 will be removed.
	FastForward(d time.Duration)
}

func freeport() (addr string, port int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fallbackAddr, fallbackPort
	}

	netaddr := listener.Addr().(*net.TCPAddr)
	addr = netaddr.String()
	port = netaddr.Port
	_ = listener.Close()
	time.Sleep(time.Second)
	return addr, port
}

// Start starts a redis-server when available, otherwise falls back to miniredis.
func Start(ctx context.Context) (Server, error) {
	server, err := Process(ctx)
	if err != nil {
		log.Println("failed to start redis-server: ", err)
		return Mini(ctx)
	}
	return server, err
}

// Process starts a redis-server test process.
func Process(ctx context.Context) (Server, error) {
	tmpdir, err := os.MkdirTemp("", "storj-redis")
	if err != nil {
		return nil, err
	}

	// find a suitable port for listening
	addr, port := freeport()

	// write a configuration file, because redis doesn't support flags
	confpath := filepath.Join(tmpdir, "test.conf")
	arguments := []string{
		"daemonize no",
		"bind 127.0.0.1",
		"port " + strconv.Itoa(port),
		"timeout 0",
		"databases 2",
		"dbfilename dump.rdb",
		"dir " + tmpdir,
	}

	conf := strings.Join(arguments, "\n") + "\n"
	err = os.WriteFile(confpath, []byte(conf), 0755)
	if err != nil {
		return nil, err
	}

	// start the process
	cmd := exec.Command("redis-server", confpath)
	processgroup.Setup(cmd)

	read, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	cmd.Stdout = write
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	cleanup := func() {
		processgroup.Kill(cmd)
		_ = os.RemoveAll(tmpdir)
	}

	// wait for redis to become ready
	waitForReady := make(chan error, 1)
	go func() {
		// wait for the message that looks like
		// v3  "The server is now ready to accept connections on port 6379"
		// v4  "Ready to accept connections"
		scanner := bufio.NewScanner(read)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "to accept") {
				break
			}
		}
		waitForReady <- scanner.Err()
		_, _ = io.Copy(io.Discard, read)
	}()

	select {
	case err := <-waitForReady:
		if err != nil {
			cleanup()
			return nil, err
		}
	case <-time.After(3 * time.Second):
		cleanup()
		return nil, errors.New("redis timeout")
	}

	// test whether we can actually connect
	if err := pingServer(ctx, addr); err != nil {
		cleanup()
		return nil, fmt.Errorf("unable to ping: %w", err)
	}

	return &process{addr, cleanup}, nil
}

type process struct {
	addr  string
	close func()
}

func (process *process) Addr() string {
	return process.addr
}

func (process *process) Close() error {
	process.close()
	return nil
}

func (process *process) FastForward(_ time.Duration) {}

func pingServer(ctx context.Context, addr string) error {
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 1})
	defer func() { _ = client.Close() }()
	return client.Ping(ctx).Err()
}

// Mini starts miniredis server.
func Mini(ctx context.Context) (Server, error) {
	var server *miniredis.Miniredis
	var err error

	pprof.Do(ctx, pprof.Labels("db", "miniredis"), func(ctx context.Context) {
		server, err = miniredis.Run()
	})

	if err != nil {
		return nil, err
	}
	return &miniserver{server}, nil
}

type miniserver struct {
	*miniredis.Miniredis
}

// Close closes the underlying miniredis server.
func (s *miniserver) Close() error {
	s.Miniredis.Close()
	return nil
}

func (s *miniserver) FastForward(d time.Duration) {
	s.Miniredis.FastForward(d)
}
