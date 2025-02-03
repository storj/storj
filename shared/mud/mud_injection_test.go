// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

type Logger struct {
	Prefix string
}

func NewLogger() Injector[*Logger] {
	return func(ball *Ball, r reflect.Type) *Logger {
		return &Logger{
			Prefix: r.String(),
		}
	}
}

func (l *Logger) Log(msg string) string {
	return l.Prefix + " " + msg
}

type S1 struct {
	Logger *Logger
}

func NewS1(logger *Logger) *S1 {
	return &S1{
		Logger: logger,
	}

}

func TestCustomInjection(t *testing.T) {
	ctx := testcontext.New(t)
	ball := NewBall()
	Provide[*S1](ball, NewS1)
	Factory[*Logger](ball, NewLogger)
	err := ForEach(ball, func(component *Component) error {
		return component.Init(ctx)
	})
	require.NoError(t, err)
	msg, err := Execute[string](ctx, ball, func(s1 *S1) string {
		return s1.Logger.Log("test")
	})
	require.NoError(t, err)
	require.Equal(t, "*mud.S1 test", msg)
}
