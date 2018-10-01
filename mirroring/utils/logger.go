package utils

import (
	"os"
	"fmt"
	"io"
)

type Logger interface {
	Log(string)
	LogE(error)
}

type lg struct {
	buffer io.Writer
	errBuffer io.Writer

	format, errFormat string
}

func (l *lg) log(msg string) {
	l.buffer.Write([]byte(fmt.Sprintf(l.format, msg)))
}

func (l *lg) logE(err error) {
	l.errBuffer.Write([]byte(fmt.Sprintf(l.errFormat, err)))
}

func (l *lg) Log(msg string) {
	if msg == "" {
		return
	}

	l.log(msg)
}

func (l *lg) LogE(err error) {
	if err == nil {
		return
	}

	l.logE(err)
}

func (l *lg) Write(b []byte) (n int, err error) {
	return l.buffer.Write(b)
}

var StdOutLogger = lg{
	buffer: os.Stdout,
	errBuffer: os.Stdout,
	format: "Log: %s\n",
	errFormat: "Err: %s\n",
}