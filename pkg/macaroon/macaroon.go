// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

type Macaroon struct {
	head    []byte
	caveats []Caveat
	tail    []byte
}

type Caveat struct {
	Identifier string
}

func NewUnrestrictedMacaroon(secret []byte) (*Macaroon, error) {
	head, err := NewNonce()
	if err != nil {
		return nil, err
	}

	return &Macaroon{
		head: head,
		tail: sign(secret, head),
	}, nil
}

func sign(secret []byte, data []byte) []byte {
	signer := hmac.New(sha256.New, secret)
	signer.Write(data)

	return signer.Sum(nil)
}

func NewSecret() (secret []byte, err error) {
	secret = make([]byte, 32)

	_, err = rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func NewNonce() (nonce []byte, err error) {
	nonce = make([]byte, 32)

	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	return nonce, nil
}

func (m *Macaroon) AddFirstPartyCaveat(c Caveat) (macaroon *Macaroon, err error) {
	macaroon = m.Copy()

	macaroon.caveats = append(macaroon.caveats, c)
	macaroon.tail = sign(macaroon.tail, []byte(c.Identifier))

	return macaroon, nil
}

func CheckUnpack(secret []byte, macaroon *Macaroon) (c []Caveat, ok bool) {
	tail := sign(secret, macaroon.head)
	for _, cav := range macaroon.caveats {
		tail = sign(tail, []byte(cav.Identifier))
	}

	if 0 == subtle.ConstantTimeCompare(tail, macaroon.tail) {
		return nil, false
	}

	return macaroon.Caveats(), true
}

func (m *Macaroon) Head() (head []byte) {
	head = make([]byte, len(m.head))
	copy(head, m.head)

	return head
}

func (m *Macaroon) Caveats() (caveats []Caveat) {
	caveats = make([]Caveat, len(m.caveats))
	copy(caveats, m.caveats)

	return caveats
}

func (m *Macaroon) Tail() (tail []byte) {
	tail = make([]byte, len(m.tail))
	copy(tail, m.tail)

	return tail
}

func (m *Macaroon) Copy() *Macaroon {
	mac := Macaroon{}

	mac.head = make([]byte, len(m.head))
	copy(mac.head, m.head)
	mac.caveats = make([]Caveat, len(m.caveats))
	copy(mac.caveats, m.caveats)
	mac.tail = make([]byte, len(m.tail))
	copy(mac.tail, m.tail)

	return &mac
}
