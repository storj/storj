// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"storj.io/storj/storage"
)

const (
	testHost     = "127.0.0.1:6379"
	testDatabase = 1
)

func TestCommon(t *testing.T) {
	cmd := exec.Command("redis-server")
	if err := cmd.Start(); os.IsNotExist(err) {
		t.Skip("redis not installed")
	}
	defer cmd.Process.Kill()

	// wait for redis to start
	time.Sleep(time.Second)

	client, err := NewClient(testHost, "", testDatabase)
	if err != nil {
		t.Fatal(err)
	}

	storage.RunTests(t, client)
}

func TestInvalidConnection(t *testing.T) {
	_, err := NewClient("", "", testDatabase)
	if err == nil {
		t.Fatal("expected connection error")
	}
}
