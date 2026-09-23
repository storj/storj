// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package post

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSMTPSenderStalled(t *testing.T) {
	for _, stage := range []string{"greeting", "command"} {
		for _, limit := range []string{"cancellation", "context deadline", "sender timeout"} {
			t.Run(stage+"/"+limit, func(t *testing.T) {
				testSMTPSenderStalled(t, stage, limit)
			})
		}
	}
}

func testSMTPSenderStalled(t *testing.T, stage, limit string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	stalled := make(chan struct{})
	serverDone := make(chan error, 1)
	release := make(chan struct{})
	defer func() {
		close(release)
		_ = listener.Close()
		<-serverDone
	}()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer func() { _ = conn.Close() }()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			serverDone <- err
			return
		}
		if stage == "command" {
			if _, err = fmt.Fprint(conn, "220 localhost ESMTP\r\n"); err == nil {
				var line string
				line, err = bufio.NewReader(conn).ReadString('\n')
				if err == nil && !strings.HasPrefix(line, "EHLO ") {
					err = fmt.Errorf("expected EHLO, got %q", line)
				}
			}
			if err != nil {
				serverDone <- err
				return
			}
		}
		close(stalled)
		<-release
		serverDone <- nil
	}()

	ctx := context.Background()
	var cancel context.CancelFunc
	if limit == "context deadline" {
		ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	sender := &SMTPSender{ServerAddress: listener.Addr().String()}
	if limit == "sender timeout" {
		sender.Timeout = 200 * time.Millisecond
	}
	done := make(chan error, 1)
	go func() { done <- sender.SendEmail(ctx, &Message{}) }()

	select {
	case <-stalled:
	case err := <-serverDone:
		serverDone <- err
		t.Fatalf("SMTP server failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("sender did not reach SMTP server")
	}
	if limit == "cancellation" {
		cancel()
	}
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatalf("SMTP send did not stop after %s", limit)
	}
}

func TestSMTPSenderDelivery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- func() error {
			conn, err := listener.Accept()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return err
			}
			protocol := textproto.NewConn(conn)
			if err := protocol.PrintfLine("220 localhost ESMTP"); err != nil {
				return err
			}
			for _, step := range []struct{ command, response string }{
				{"EHLO localhost", "250 localhost"},
				{"MAIL FROM:<sender@example.com>", "250 OK"},
				{"RCPT TO:<recipient@example.com>", "250 OK"},
				{"DATA", "354 Send message"},
			} {
				line, err := protocol.ReadLine()
				if err != nil {
					return err
				}
				if line != step.command {
					return fmt.Errorf("expected %q, got %q", step.command, line)
				}
				if err := protocol.PrintfLine("%s", step.response); err != nil {
					return err
				}
			}
			data, err := protocol.ReadDotBytes()
			if err != nil {
				return err
			}
			if !strings.Contains(string(data), "Subject: Test delivery") {
				return fmt.Errorf("message subject missing")
			}
			if err := protocol.PrintfLine("250 Queued"); err != nil {
				return err
			}
			line, err := protocol.ReadLine()
			if err != nil {
				return err
			}
			if line != "QUIT" {
				return fmt.Errorf("expected QUIT, got %q", line)
			}
			return protocol.PrintfLine("221 Bye")
		}()
	}()

	sender := &SMTPSender{
		ServerAddress: listener.Addr().String(),
		From:          Address{Address: "sender@example.com"},
	}
	err = sender.SendEmail(context.Background(), &Message{
		From:      sender.From,
		To:        []Address{{Address: "recipient@example.com"}},
		Subject:   "Test delivery",
		PlainText: "Hello",
	})
	_ = listener.Close()
	require.NoError(t, <-serverDone)
	require.NoError(t, err)
}
