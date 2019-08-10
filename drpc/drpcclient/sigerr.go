// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import "sync"

type sigerr struct {
	once sync.Once
	sig  chan struct{}
	err  error
}

func newSigerr() sigerr {
	return sigerr{
		sig: make(chan struct{}),
		err: nil,
	}
}

func (s *sigerr) signalWithError(err error) {
	s.once.Do(func() {
		s.err = err
		close(s.sig)
	})
}

func (s *sigerr) wasSignaled() bool {
	select {
	case <-s.sig:
		return true
	default:
		return false
	}
}

func (s *sigerr) pollError() error {
	select {
	case <-s.sig:
		return s.err
	default:
		return nil
	}
}
