// Copyright (C) 2019 Storj Labs, Inc. // See LICENSE for copying information.

package macaroon

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

// Macaroon ...
type Macaroon struct {
	head    []byte
	caveats []Caveat
	tail    []byte
}

// Caveat ...
type Caveat struct {
	Identifier string
}

// NewUnrestricted creates Macaroon with random Head and generated Tail
func NewUnrestricted(identifier []byte, secret []byte) *Macaroon {
	return &Macaroon{
		head: identifier,
		tail: sign(secret, identifier),
	}
}

// sign
func sign(secret []byte, data []byte) []byte {
	signer := hmac.New(sha256.New, secret)
	// Error skipped because sha256 does not return error
	_, _ = signer.Write(data)

	return signer.Sum(nil)
}

// NewSecret generates cryptographically random 32 bytes
func NewSecret() (secret []byte, err error) {
	secret = make([]byte, 32)

	_, err = rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// NewNonce generates cryptographically random 32 bytes
func NewNonce() (nonce []byte, err error) {
	nonce = make([]byte, 32)

	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	return nonce, nil
}

// AddFirstPartyCaveat creates signed macaroon with appended caveat
func (m *Macaroon) AddFirstPartyCaveat(c Caveat) (macaroon *Macaroon, err error) {
	macaroon = m.Copy()

	macaroon.caveats = append(macaroon.caveats, c)
	macaroon.tail = sign(macaroon.tail, []byte(c.Identifier))

	return macaroon, nil
}

// CheckUnpack reconstructs with all caveats from the secret and compares tails.
// Returns list of Caveats if tails matches.
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

// Head returns copy of macaroon head
func (m *Macaroon) Head() (head []byte) {
	if len(m.head) == 0 {
		return nil
	}

	head = make([]byte, len(m.head))
	copy(head, m.head)

	return head
}

// Caveats returns copy of macaroon caveats
func (m *Macaroon) Caveats() (caveats []Caveat) {
	if len(m.caveats) == 0 {
		return nil
	}

	caveats = make([]Caveat, len(m.caveats))
	copy(caveats, m.caveats)

	return caveats
}

// Tail returns copy of macaroon tail
func (m *Macaroon) Tail() (tail []byte) {
	if len(m.tail) == 0 {
		return nil
	}

	tail = make([]byte, len(m.tail))
	copy(tail, m.tail)

	return tail
}

// Copy return copy of macaroon
func (m *Macaroon) Copy() *Macaroon {
	return &Macaroon{
		head:    m.Head(),
		caveats: m.Caveats(),
		tail:    m.Tail(),
	}
}
