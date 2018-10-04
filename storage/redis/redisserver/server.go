// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Package redisserver is package for starting a redis test server
package redisserver

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
	"github.com/zeebo/errs"
	"storj.io/storj/internal/processgroup"
)

var (
	// Error is a redis error
	Error = errs.Class("redis server error")
)

const (
	fallbackAddr = "localhost:6379"
	fallbackPort = 6379
)

func freeport(inport int) (addr string, port int, err error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", inport))
	if err != nil {
		if inport == 0 {
			return fallbackAddr, fallbackPort, nil
		}

		return "", 0, Error.Wrap(err)
	}

	netaddr := listener.Addr().(*net.TCPAddr)
	addr = netaddr.String()
	port = netaddr.Port
	_ = listener.Close()
	time.Sleep(time.Second)
	return addr, port, nil
}

// Start starts a redis-server when available, otherwise falls back to miniredis
func Start() (addr string, cleanup func(), err error) {
	addr, cleanup, err = Process()
	if err != nil {
		log.Println("failed to start redis-server: ", err)
		return Mini()
	}
	return addr, cleanup, err
}

// Start starts a redis-server at the specified port, otherwise falls back to miniredis
func StartAt(port int) (addr string, cleanup func(), err error) {
	return ProcessAt(port)
}

func Process() (addr string, cleanup func(), err error) {
	return ProcessAt(0)
}

// Process starts a redis-server test process
func ProcessAt(port int) (addr string, cleanup func(), err error) {
	tmpdir, err := ioutil.TempDir("", "storj-redis")
	if err != nil {
		return "", nil, err
	}

	// find a suitable port for listening
	addr, port, err = freeport(port)
	if err != nil {
		return "", nil, err
	}

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
	err = ioutil.WriteFile(confpath, []byte(conf), 0755)
	if err != nil {
		return "", nil, err
	}

	// start the process
	cmd := exec.Command("redis-server", confpath)
	processgroup.Setup(cmd)

	read, write, err := os.Pipe()
	if err != nil {
		return "", nil, err
	}

	cmd.Stdout = write
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}

	cleanup = func() {
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
		_, _ = io.Copy(ioutil.Discard, read)
	}()

	select {
	case err := <-waitForReady:
		if err != nil {
			cleanup()
			return "", nil, err
		}
	case <-time.After(3 * time.Second):
		cleanup()
		return "", nil, errors.New("redis timeout")
	}

	// test whether we can actually connect
	if err := pingServer(addr); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("unable to ping: %v", err)
	}

	return addr, cleanup, nil
}

func pingServer(addr string) error {
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 1})
	defer func() { _ = client.Close() }()
	return client.Ping().Err()
}

// Mini starts miniredis server
func Mini() (addr string, cleanup func(), err error) {
	server, err := miniredis.Run()
	if err != nil {
		return "", nil, err
	}

	return server.Addr(), func() {
		server.Close()
	}, nil
}
