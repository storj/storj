// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import "strings"

// Code is an enumeration of states some billing entry could be in.
type Code string

const (
	// Disqualified is included if the node is disqualified.
	Disqualified Code = "D"

	// Sanctioned is included if payment is withheld because the node is in
	// a sanctioned country.
	Sanctioned Code = "S"

	// No1099 is included if payment is withheld because the node has not
	// filed a 1099 and payment would put it over limits.
	No1099 Code = "T"

	// InWithholding is included if the node is in the initial held amount
	// period.
	InWithholding Code = "E"

	// GracefulExit is included if the node has gracefully exited.
	GracefulExit Code = "X"

	// Offline is included if the node's last contact success is before the starting
	// period.
	Offline Code = "O"

	// Bonus is included if the node has qualified for special bonus circumstances,
	// chosen month by month by the crypthopper-go accountant prepare step.
	Bonus Code = "B"
)

// CodeFromString parses the string into a Code.
func CodeFromString(s string) (Code, error) {
	code := Code(s)
	switch code {
	case Disqualified, Sanctioned, No1099, InWithholding, GracefulExit, Offline, Bonus:
		return code, nil
	default:
		return "", Error.New("no such code %q", code)
	}
}

// Codes represents a collection of Code values.
type Codes []Code

// String serializes the Codes into a colon separated list.
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

// UnmarshalCSV does the custom unmarshaling of Codes.
func (codes *Codes) UnmarshalCSV(s string) error {
	value, err := CodesFromString(s)
	if err != nil {
		return err
	}
	*codes = value
	return nil
}

// MarshalCSV does the custom marshaling of Codes.
func (codes Codes) MarshalCSV() (string, error) {
	return codes.String(), nil
}

// CodesFromString parses the list of codes into a Codes.
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
