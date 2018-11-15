// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"net"
	"sync"
	"time"
)

// UDPSource is a packet source
type UDPSource struct {
	mtx     sync.Mutex
	address string
	conn    *net.UDPConn
	buf     [1024 * 10]byte
}

// NewUDPSource creates a UDPSource that listens on address
func NewUDPSource(address string) *UDPSource {
	return &UDPSource{address: address}
}

// Next implements the Source interface
func (s *UDPSource) Next() ([]byte, time.Time, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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

// UDPDest is a packet destination. IMPORTANT: It throws away timestamps.
type UDPDest struct {
	mtx     sync.Mutex
	address string
	addr    *net.UDPAddr
	conn    *net.UDPConn
}

// NewUDPDest creates a UDPDest that sends incoming packets to address.
func NewUDPDest(address string) *UDPDest {
	return &UDPDest{address: address}
}

// Packet implements PacketDest
func (d *UDPDest) Packet(data []byte, ts time.Time) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.conn == nil {
		addr, err := net.ResolveUDPAddr("udp", d.address)
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
		if err != nil {
			return err
		}
		d.addr = addr
		d.conn = conn
	}

	_, err := d.conn.WriteTo(data, d.addr)
	return err
}
