// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcutil

import "sync"

type Signal struct {
	once sync.Once
	sig  chan struct{}
	err  error
}

func NewSignal() *Signal {
	return &Signal{
		sig: make(chan struct{}),
		err: nil,
	}
}

func (s *Signal) Signal() chan struct{} {
	return s.sig
}

func (s *Signal) SignalWithError(err error) {
	s.once.Do(func() {
		s.err = err
		close(s.sig)
	})
}

func (s *Signal) WasSignaled() bool {
	select {
	case <-s.sig:
		return true
	default:
		return false
	}
}

func (s *Signal) Err() error {
	select {
	case <-s.sig:
		return s.err
	default:
		return nil
	}
}
