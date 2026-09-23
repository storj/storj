// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package post

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

// Address is alias of net/mail.Address.
type Address = mail.Address

var mon = monkit.Package()

// SMTPSender is smtp sender.
type SMTPSender struct {
	ServerAddress string

	From Address
	Auth smtp.Auth

	// Timeout bounds the SMTP exchange, including dialing. A non-positive value
	// uses 30 seconds. An earlier context deadline takes precedence.
	Timeout time.Duration
}

// AuthWithContext is implemented by an smtp.Auth that contacts an external
// service, such as an oauth2 token endpoint, and can bound it by the send's
// context.
type AuthWithContext interface {
	WithContext(ctx context.Context) smtp.Auth
}

// FromAddress implements satellite/mail.SMTPSender.
func (sender *SMTPSender) FromAddress() Address {
	return sender.From
}

// SendEmail sends email message to the given recipient.
func (sender *SMTPSender) SendEmail(ctx context.Context, msg *Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	timeout := sender.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", sender.ServerAddress)
	if err != nil {
		return err
	}
	// Quit, client.Close, or cancellation may already have closed the connection.
	defer func() { _ = conn.Close() }()

	// A deadline bounds SMTP reads, writes, and TLS handshakes. Closing the
	// connection also interrupts them when the caller cancels before that deadline.
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	host, _, err := net.SplitHostPort(sender.ServerAddress)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}

	if err = sender.communicate(ctx, client, msg); err != nil {
		// The socket error caused by closing the connection on cancellation says
		// nothing useful; report why the send was given up on instead. The
		// deferred conn.Close covers the client in that case.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return errs.Combine(err, client.Close())
	}

	return nil
}

// communicate sends mail via SMTP using provided client and message.
func (sender *SMTPSender) communicate(ctx context.Context, client *smtp.Client, msg *Message) error {
	// suppress error because address should be validated
	// before creating SMTPSender
	host, _, _ := net.SplitHostPort(sender.ServerAddress)

	if sender.Auth != nil {
		// send smtp hello or ehlo msg and establish connection over tls
		err := client.StartTLS(&tls.Config{ServerName: host})
		if err != nil {
			return err
		}

		auth := sender.Auth
		if withCtx, ok := auth.(AuthWithContext); ok {
			auth = withCtx.WithContext(ctx)
		}

		err = client.Auth(auth)
		if err != nil {
			return err
		}
	}

	err := client.Mail(sender.From.Address)
	if err != nil {
		return err
	}

	// add recipients
	for _, to := range msg.To {
		err = client.Rcpt(to.Address)
		if err != nil {
			return err
		}
	}

	mess, err := msg.Bytes()
	if err != nil {
		return err
	}

	data, err := client.Data()
	if err != nil {
		return err
	}

	err = writeData(data, mess)
	if err != nil {
		return err
	}

	// send quit msg to stop gracefully
	return client.Quit()
}

// writeData ensures that writer will be closed after data is written.
func writeData(writer io.WriteCloser, data []byte) (err error) {
	defer func() {
		err = errs.Combine(err, writer.Close())
	}()

	_, err = writer.Write(data)
	return
}
