// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"crypto/tls"
	"net"
	"net/smtp"

	"github.com/zeebo/errs"
)

// SmtpServer is smtp server
type SmtpServer struct {
	Host, Port string
	Sender     Sender
}

// Address returns server's network address
func (s *SmtpServer) Address() string {
	return net.JoinHostPort(s.Host, s.Port)
}

// SendEmail sends email message to the given recipient
func (server *SmtpServer) SendEmail(rcpt string, msg []byte) error {
	client, err := smtp.Dial(server.Address())
	if err != nil {
		return err
	}
	// close underlying connection
	defer func() {
		err = errs.Combine(err, client.Close())
	}()

	// send smtp hello or ehlo msg and establish connection over tls
	err = client.StartTLS(&tls.Config{ServerName: server.Host})
	if err != nil {
		return err
	}

	err = client.Auth(server.Sender.Auth)
	if err != nil {
		return err
	}

	err = client.Mail(server.Sender.Mail)
	if err != nil {
		return err
	}

	err = client.Rcpt(rcpt)
	if err != nil {
		return err
	}

	data, err := client.Data()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, data.Close())
	}()

	_, err = data.Write(msg)
	return err
}
