// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Package redisserver is package for starting a redis test server
package redisserver

import (
	"bufio"
	"bytes"
	"errors"
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
)

const (
	fallbackAddr = "localhost:3780"
	fallbackPort = 3780
)

func freeport() (addr string, port int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fallbackAddr, fallbackPort
	}

	addr = listener.Addr().String()
	port = listener.Addr().(*net.TCPAddr).Port

	_ = listener.Close()
	return addr, port
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

// Process starts a redis-server test process
func Process() (addr string, cleanup func(), err error) {
	tmpdir, err := ioutil.TempDir("", "storj-redis")
	if err != nil {
		return "", nil, err
	}

	// find a suitable port for listening
	var port int
	addr, port = freeport()

	// write a configuration file, because redis doesn't support flags
	confpath := filepath.Join(tmpdir, "test.conf")
	arguments := []string{
		"daemonize no",
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
	var redisout bytes.Buffer
	cmd.Stdout = &redisout
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}

	cleanup = func() {
		_ = cmd.Process.Kill()
		_ = os.RemoveAll(tmpdir)
	}

	// wait for redis to become ready
	waitForReady := make(chan struct{}, 5)
	go func() {
		// wait for the message that looks like
		//   "The server is now ready to accept connections on port 6379"
		scanner := bufio.NewScanner(&redisout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "now ready to accept") {
				break
			}
		}
		waitForReady <- struct{}{}
		close(waitForReady)
		_, _ = io.Copy(ioutil.Discard, &redisout)
	}()

	select {
	case <-waitForReady:
	case <-time.After(3 * time.Second):
		cleanup()
		return "", nil, errors.New("redis timeout")
	}

	// test whether we can actually connect
	if !pingServer(addr) {
		cleanup()
		return "", nil, errors.New("unable to ping")
	}

	return addr, cleanup, nil
}

func pingServer(addr string) bool {
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	defer func() { _ = client.Close() }()
	return client.Ping().Err() == nil
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
