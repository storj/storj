// Copyright (C) 2019 Storj Labs, Inc.
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

// Scope represents a collection of values registered in a Lua namespace.
type Scope struct {
	mu            sync.Mutex
	registrations map[string]func(*lua.State) error
}

// NewScope creates an empty Scope.
func NewScope() *Scope {
	return &Scope{
		registrations: map[string]func(*lua.State) error{},
	}
}

// RegisterType allows you to add a Lua function that creates new
// values of the given type to the scope.
func (scope *Scope) RegisterType(name string, example interface{}) error {
	return scope.register(name, example, luar.PushType)
}

// RegisterVal adds the Go value 'value', including Go functions, to the Lua
// scope.
func (scope *Scope) RegisterVal(name string, value interface{}) error {
	return scope.register(name, value, luar.PushValue)
}

func (scope *Scope) register(name string, val interface{}, pusher func(l *lua.State, val interface{}) error) error {
	scope.mu.Lock()
	defer scope.mu.Unlock()

	if _, exists := scope.registrations[name]; exists {
		return fmt.Errorf("Registration %#v already exists", name)
	}

	scope.registrations[name] = func(l *lua.State) error {
		err := pusher(l, val)
		if err != nil {
			return err
		}
		l.SetGlobal(name)
		return nil
	}
	return nil
}

// Run runs the Lua source represented by in
func (scope *Scope) Run(in io.Reader) error {
	l := lua.NewState()
	luar.SetOptions(l, luar.Options{AllowUnexportedAccess: true})

	scope.mu.Lock()
	registrations := make([]func(l *lua.State) error, 0, len(scope.registrations))
	for _, reg := range scope.registrations {
		registrations = append(registrations, reg)
	}
	scope.mu.Unlock()

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
