// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"crypto/tls"
	"net"
	"net/smtp"

	"github.com/zeebo/errs"
)

// SMTPSender is smtp server
type SMTPSender struct {
	ServerAddress string

	From string
	Auth smtp.Auth
}

// SendEmail sends email message to the given recipient
func (sender *SMTPSender) SendEmail(rcpt string, msg []byte) error {
	host, _, err := net.SplitHostPort(sender.ServerAddress)
	if err != nil {
		return err
	}

	client, err := smtp.Dial(sender.ServerAddress)
	if err != nil {
		return err
	}
	// close underlying connection
	defer func() {
		err = errs.Combine(err, client.Close())
	}()

	// send smtp hello or ehlo msg and establish connection over tls
	err = client.StartTLS(&tls.Config{ServerName: host})
	if err != nil {
		return err
	}

	err = client.Auth(sender.Auth)
	if err != nil {
		return err
	}

	err = client.Mail(sender.From)
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
	if err != nil {
		return err
	}

	return client.Quit()
}
