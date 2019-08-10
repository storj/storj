// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"context"
	"io"
	"os"
	"testing"

	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcwire"
)

func TestClient(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	client := New(struct {
		io.ReadCloser
		io.Writer
	}{
		ReadCloser: pr,
		Writer:     drpcwire.NewDumper(os.Stdout),
	})
	defer client.Close()

	t.Log(client.Invoke(context.Background(), "test", mockMsg{}, nil))
}

type mockMsg struct{ drpc.Message }

func (m mockMsg) Marshal() ([]byte, error) { return []byte("foo"), nil }
