// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testing

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TLog = zap.Logger

type T struct {
	name   string
	logger *TLog
	f      func(*T)
}

type Tests []T

var defaultLogger *zap.Logger

func init() {
	// TODO: pass/fail log output
	logConfig := zap.NewDevelopmentConfig()
	logConfig.EncoderConfig = zapcore.EncoderConfig{
		NameKey:     "N",
		MessageKey:  "M",
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
	}

	var err error
	defaultLogger, err = logConfig.Build()
	if err != nil {
		panic(err)
	}
}

func NewTest(name string, f func(*T)) T {
	return T{
		name:   name,
		logger: defaultLogger.Named(name),
		f:      f,
	}
}

func (tests *Tests) Register(newTests ...T) {
	*tests = append(*tests, newTests...)
}

func (tests Tests) Run() {
	for _, t := range tests {
		t.Run()
	}
}

func (t T) Run() {
	t.f(&t)
}

func (t T) FailNow() {
	panic("FailNow called")
}

func (t T) Debug(msg string, fields ...zap.Field) {
	t.logger.Debug(msg, fields...)
}

func (t T) Info(msg string, fields ...zap.Field) {
	t.logger.Info(msg, fields...)
}

func (t T) Warn(msg string, fields ...zap.Field) {
	t.logger.Warn(msg, fields...)
}

func (t T) Error(msg string, fields ...zap.Field) {
	t.logger.Error(msg, fields...)
}

func (t T) Errorf(template string, args ...interface{}) {
	t.logger.Error(fmt.Sprintf(template, args...))
}
