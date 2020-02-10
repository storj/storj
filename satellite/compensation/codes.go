// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import "strings"

type Code string

const (
	Disqualified Code = "D"
	Sanctioned   Code = "S"
	No1099       Code = "T"
	InEscrow     Code = "E"
	GracefulExit Code = "X"
)

func CodeFromString(s string) (Code, error) {
	code := Code(s)
	switch code {
	case Disqualified, Sanctioned, No1099, InEscrow, GracefulExit:
		return code, nil
	default:
		return "", Error.New("no such code %q", code)
	}
}

type Codes []Code

func (codes Codes) String() string {
	builder := new(strings.Builder)
	for i, code := range codes {
		if i > 0 {
			builder.WriteByte(':')
		}
		builder.WriteString(string(code))
	}
	return builder.String()
}

func (codes *Codes) UnmarshalCSV(s string) error {
	value, err := CodesFromString(s)
	if err != nil {
		return err
	}
	*codes = value
	return nil
}

func (codes Codes) MarshalCSV() (string, error) {
	return codes.String(), nil
}

func CodesFromString(s string) (codes Codes, err error) {
	for _, segment := range strings.Split(s, ":") {
		if len(segment) == 0 {
			// ignore empty segments
			continue
		}
		code, err := CodeFromString(segment)
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}
