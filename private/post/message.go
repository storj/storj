// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package post

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"time"

	"github.com/zeebo/errs"
)

// Message is RFC compliant email message.
type Message struct {
	From      Address
	To        []Address
	Subject   string
	ID        string
	Date      time.Time
	ReceiptTo []string

	PlainText string
	Parts     []Part
}

// Part represent one part of multipart message.
type Part struct {
	Type        string
	Encoding    string
	Disposition string
	Content     string
}

// Error is the default message errs class.
var Error = errs.Class("Email message")

// Bytes builds message and returns result as bytes.
func (msg *Message) Bytes() (data []byte, err error) {
	// always returns nil error on read and write, so most of the errors can be ignored
	var body bytes.Buffer

	// write headers
	_, _ = fmt.Fprintf(&body, "Subject: %v\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	_, _ = fmt.Fprintf(&body, "From: %s\r\n", &msg.From)
	for _, to := range msg.To {
		_, _ = fmt.Fprintf(&body, "To: %s\r\n", &to) //nolint:scopelint
	}
	for _, recipient := range msg.ReceiptTo {
		_, _ = fmt.Fprintf(&body, "Disposition-Notification-To: <%v>\r\n", mime.QEncoding.Encode("utf-8", recipient))
	}
	// date and id are optional as they can be set by server itself
	if !msg.Date.IsZero() {
		_, _ = fmt.Fprintf(&body, "Date: %v\r\n", msg.Date)
	}
	if msg.ID != "" {
		_, _ = fmt.Fprintf(&body, "Message-ID: <%v>\r\n", mime.QEncoding.Encode("utf-8", msg.ID))
	}
	_, _ = fmt.Fprintf(&body, "MIME-Version: 1.0\r\n")

	switch {
	// multipart upload
	case len(msg.Parts) > 0:
		err = msg.writeMultipart(&body)
		if err != nil {
			return nil, err
		}

	// fallback if there are no parts, write PlainText with appropriate Content-Type
	default:
		_, _ = fmt.Fprintf(&body, "Content-Type: text/plain; charset=UTF-8; format=flowed\r\n")
		_, _ = fmt.Fprintf(&body, "Content-Transfer-Encoding: quoted-printable\r\n\r\n")

		enc := quotedprintable.NewWriter(&body)
		defer func() { err = errs.Combine(err, enc.Close()) }()

		if _, err := enc.Write([]byte(msg.PlainText)); err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return tocrlf(body.Bytes()), nil
}

func (msg *Message) writeMultipart(body *bytes.Buffer) (err error) {
	wr := multipart.NewWriter(body)
	defer func() { err = errs.Combine(err, wr.Close()) }()

	_, _ = fmt.Fprintf(body, "Content-Type: multipart/alternative;")
	_, _ = fmt.Fprintf(body, "\tboundary=\"%v\"\r\n", wr.Boundary())
	_, _ = fmt.Fprintf(body, "\r\n")

	var sub io.Writer

	if len(msg.PlainText) > 0 {
		sub, err := wr.CreatePart(textproto.MIMEHeader{
			"Content-Type":              []string{"text/plain; charset=UTF-8; format=flowed"},
			"Content-Transfer-Encoding": []string{"quoted-printable"},
		})
		if err != nil {
			return Error.Wrap(err)
		}

		enc := quotedprintable.NewWriter(sub)
		defer func() { err = errs.Combine(err, enc.Close()) }()

		_, err = enc.Write([]byte(msg.PlainText))
		if err != nil {
			return Error.Wrap(err)
		}
	}

	for _, part := range msg.Parts {
		header := textproto.MIMEHeader{"Content-Type": []string{mime.QEncoding.Encode("utf-8", part.Type)}}
		if part.Encoding != "" {
			header["Content-Transfer-Encoding"] = []string{mime.QEncoding.Encode("utf-8", part.Encoding)}
		}
		if part.Disposition != "" {
			header["Content-Disposition"] = []string{mime.QEncoding.Encode("utf-8", part.Disposition)}
		}

		sub, _ = wr.CreatePart(header)
		_, _ = fmt.Fprint(sub, part.Content)
	}
	return nil
}

func tocrlf(data []byte) []byte {
	lf := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	return crlf
}
