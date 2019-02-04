// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// UDPSource is a packet source
type UDPSource struct {
	address string

	mu     sync.Mutex
	conn   *net.UDPConn
	buf    [1024 * 10]byte
	closed bool
}

// NewUDPSource creates a UDPSource that listens on address
func NewUDPSource(address string) *UDPSource {
	return &UDPSource{address: address}
}

// Next implements the Source interface
func (s *UDPSource) Next() ([]byte, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, time.Time{}, fmt.Errorf("udp source closed")
	}
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

// Close closes the source
func (s *UDPSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// UDPDest is a packet destination. IMPORTANT: It throws away timestamps.
type UDPDest struct {
	address string

	mu     sync.Mutex
	addr   *net.UDPAddr
	conn   *net.UDPConn
	closed bool
}

// NewUDPDest creates a UDPDest that sends incoming packets to address.
func NewUDPDest(address string) *UDPDest {
	return &UDPDest{address: address}
}

// Packet implements PacketDest
func (d *UDPDest) Packet(data []byte, ts time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return fmt.Errorf("closed destination")
	}

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

// Close closes the destination
func (d *UDPDest) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.closed = true
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}
