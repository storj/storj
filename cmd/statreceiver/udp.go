// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"net"
	"time"
)

type UDPSource struct {
	address string
	conn    *net.UDPConn
	buf     [1024 * 10]byte
}

func NewUDPSource(address string) *UDPSource {
	return &UDPSource{address: address}
}

func (s *UDPSource) Next() ([]byte, time.Time, error) {
	if s.conn == nil {
		addr, err := net.ResolveUDPAddr("udp", s.address)
		if err != nil {
			return nil, time.Time{}, err
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return nil, time.Time{}, err
		}
		s.conn = conn
	}

	n, _, err := s.conn.ReadFrom(s.buf[:])
	if err != nil {
		return nil, time.Time{}, err
	}
	return s.buf[:n], time.Now(), nil
}
