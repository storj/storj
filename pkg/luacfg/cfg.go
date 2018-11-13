// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package luacfg

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	lua "github.com/Shopify/go-lua"
	luar "github.com/jtolds/go-luar"
)

type Scope struct {
	mtx           sync.Mutex
	registrations map[string]func(*lua.State) error
}

func NewScope() *Scope {
	return &Scope{
		registrations: map[string]func(*lua.State) error{},
	}
}

func (s *Scope) RegisterType(name string, example interface{}) error {
	return s.register(name, example, luar.PushType)
}

func (s *Scope) RegisterVal(name string, value interface{}) error {
	return s.register(name, value, luar.PushValue)
}

func (s *Scope) register(name string, val interface{},
	pusher func(l *lua.State, val interface{}) error) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, exists := s.registrations[name]; exists {
		return fmt.Errorf("Registration %#v already exists", name)
	}
	s.registrations[name] = func(l *lua.State) error {
		err := pusher(l, val)
		if err != nil {
			return err
		}
		l.SetGlobal(name)
		return nil
	}
	return nil
}

func (s *Scope) Run(in io.Reader) error {
	l := lua.NewState()
	luar.SetOptions(l, luar.Options{AllowUnexportedAccess: true})

	s.mtx.Lock()
	registrations := make([]func(l *lua.State) error, 0, len(s.registrations))
	for _, reg := range s.registrations {
		registrations = append(registrations, reg)
	}
	s.mtx.Unlock()

	for _, reg := range registrations {
		err := reg(l)
		if err != nil {
			return err
		}
	}

	data, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	err = lua.DoString(l, string(data))
	return err
}
