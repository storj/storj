// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// redisserver is package for starting a redis test server
package redisserver

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alicebob/miniredis"
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
func Start() (addr string, shutdown func(), err error) {
	addr, shutdown, err = Process()
	if err != nil {
		return Mini()
	}
	return addr, shutdown, err
}

// Process starts a redis-server test process
func Process() (addr string, shutdown func(), err error) {
	tmpdir, err := ioutil.TempDir("", "storj-redis")
	if err != nil {
		return "", nil, err
	}

	var port int
	addr, port = freeport()
	confpath := filepath.Join(tmpdir, "test.conf")

	{ // write a configuration file
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
	}

	cmd := exec.Command("redis-server", confpath)
	var redisout bytes.Buffer
	cmd.Stdout = &redisout
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}

	waitForReady := make(chan struct{}, 5)
	go func() {
		// wait for the message that looks like
		//   "The server is now ready to accept connections on port 6379"
		scanner := bufio.NewScanner(&redisout)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "ready") {
				break
			}
		}
		waitForReady <- struct{}{}
		close(waitForReady)
		io.Copy(ioutil.Discard, &redisout)
	}()

	select {
	case <-waitForReady:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		return "", nil, errors.New("redis timeout")
	}

	return addr, func() {
		cmd.Process.Kill()
		os.RemoveAll(tmpdir)
	}, nil
}

// Mini starts miniredis server
func Mini() (addr string, shutdown func(), err error) {
	server, err := miniredis.Run()
	if err != nil {
		return "", nil, err
	}

	return server.Addr(), func() {
		server.Close()
	}, nil
}
