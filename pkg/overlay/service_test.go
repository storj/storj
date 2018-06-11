// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
	"encoding/hex"
	"crypto/rand"
	"time"
	"flag"
	"testing"
	"net"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"github.com/zeebo/errs"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

type TestRedisDone func()
type TestRedisServer struct {
	cmd *exec.Cmd
	started bool
}

var (
	redisRefs = map[string]bool{}
	testRedis = &TestRedisServer{
		cmd:     &exec.Cmd{},
		started: false,
	}
)

func TestNewServer(t *testing.T) {
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := NewServer(nil, nil, nil, nil)
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {
	//a := "35.232.202.229:8080"
	//c, err := NewClient(&a, grpc.WithInsecure())
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv := NewServer(nil, nil, nil, nil)
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess(t *testing.T) {
	done := ensureRedis()
	defer done()

	o := Service{}
	ctx, _ := context.WithTimeout(context.Background(), 1 * time.Second)
	err := o.Process(ctx)
	assert.NoError(t, err)
}

func ensureRedis() (_ TestRedisDone) {
	flag.Set("redisAddress", "127.0.0.1:6379")

	index, _ := randomHex(5)
	redisRefs[index] = true

	if testRedis.started != true {
		testRedis.start()
	}

	return func () {
		if v := recover(); v != nil {
			testRedis.stop()
			panic(v)
		}

		redisRefs[index] = false

		if !(redisRefCount() > 0) {
			testRedis.stop()
		}
	}
}

func redisRefCount() (_ int) {
  count := 0
	for _, ref := range redisRefs {
		if ref {
			count += 1
		}
	}

	return count
}

func (r *TestRedisServer) start() {
	cmd := r.cmd

	logPath, err := filepath.Abs("test_redis-server.log")
	if err != nil {
		panic(err)
	}

	binPath, err := exec.LookPath("redis-server")
	if err != nil {
		panic(err)
	}

	log, err := os.Create(logPath)
	if err != nil {
		panic(err)
	}

	cmd.Path = binPath
	cmd.Stdout = log

	go func() {
    r.started = true

		if err := cmd.Run(); err != nil {
			panic(errs.New("Couldn't start redis-server: %s", err.Error()))
		}
	}()

	fmt.Printf("starting redis; sleeping")
	for range []int{0,1} {
		fmt.Printf(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Printf("done\n")
}

func (r *TestRedisServer) stop() {
	if err := r.cmd.Process.Kill(); err != nil {
		panic(err)
	}
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
