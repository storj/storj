// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"crypto/tls"
	"net"
	"net/mail"
	"net/smtp"

	"github.com/zeebo/errs"
)

// SMTPSender is smtp sender
type SMTPSender struct {
	ServerAddress string

	From mail.Address
	Auth smtp.Auth
}

// SendEmail sends email message to the given recipient
func (sender *SMTPSender) SendEmail(msg *Message) error {
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

	err = client.Mail(sender.From.Address)
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

	data, err := client.Data()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, data.Close())
	}()

	_, err = data.Write(msg.Bytes())
	if err != nil {
		return err
	}

	return client.Quit()
}
