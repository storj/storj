// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package post

import (
	"crypto/tls"
	"net"
	"net/mail"
	"net/smtp"

	"github.com/zeebo/errs"
)

// Address is alias of net/mail.Address
type Address = mail.Address

// SMTPSender is smtp sender
type SMTPSender struct {
	ServerAddress string

	From Address
	Auth smtp.Auth
}

// FromAddress implements satellite/mail.SMTPSender
func (sender *SMTPSender) FromAddress() Address {
	return sender.From
}

// SendEmail sends email message to the given recipient
func (sender *SMTPSender) SendEmail(msg *Message) error {
	// TODO: validate address before initializing SMTPSender
	// suppress error because address should be validated
	// before creating SMTPSender
	host, _, _ := net.SplitHostPort(sender.ServerAddress)

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

	// send quit msg to stop gracefully returns err on
	// success but we don't really care about the result
	_ = client.Quit()
	return nil
}
