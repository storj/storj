// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"errors"
	"fmt"
	"strings"
)

// SplitSQLStatements splits a string into SQL statements, honoring string
// literals and comments. The semicolons used to separate statements are not
// included in the returned statements.
//
// Comments are stripped from the returned statements, as in some cases Spanner
// can't parse them. Maybe it's just the emulator?  They're documented as being
// supported.
//
// Empty statements (consisting of only whitespace) are also stripped.
// Whitespace in other respects is maintained.
func SplitSQLStatements(s string) (statements []string, err error) {
	type sqlParseState int
	const (
		stateNormal sqlParseState = iota
		stateInLiteral
		stateInComment
		stateInCStyleComment
	)

	var state sqlParseState
	var literalStart byte
	var b strings.Builder

	for i := 0; i < len(s); i++ {
		switch state {
		case stateNormal:
			switch s[i] {
			case ';':
				stmt := b.String()
				if strings.TrimSpace(stmt) != "" {
					statements = append(statements, stmt)
				}
				b.Reset()
				continue
			case '\'', '"', '`':
				state = stateInLiteral
				literalStart = s[i]
			case '-':
				if i+1 < len(s) && s[i+1] == '-' {
					state = stateInComment
					i++
					continue
				}
			case '#':
				state = stateInComment
				continue
			case '/':
				if i+1 < len(s) && s[i+1] == '*' {
					state = stateInCStyleComment
					i++
					continue
				}
			}
		case stateInLiteral:
			switch s[i] {
			case literalStart:
				state = stateNormal
			case '\\':
				if i+1 < len(s) {
					b.WriteByte(s[i])
					i++
				}
			}
		case stateInComment:
			switch s[i] {
			case '\n':
				state = stateNormal
			default:
				continue
			}
		case stateInCStyleComment:
			if s[i] == '*' {
				if i+1 < len(s) && s[i+1] == '/' {
					state = stateNormal
					i++
				}
			}
			continue
		}
		b.WriteByte(s[i])
	}

	switch state {
	case stateInLiteral:
		return nil, fmt.Errorf("unterminated %q literal", rune(literalStart))
	case stateInCStyleComment:
		return nil, errors.New("unterminated C-style comment")
	}
	stmt := b.String()
	if strings.TrimSpace(stmt) != "" {
		statements = append(statements, stmt)
	}
	return statements, nil
}

// MustSplitSQLStatements works like SplitSQLStatements, but panics on error.
func MustSplitSQLStatements(s string) (statements []string) {
	statements, err := SplitSQLStatements(s)
	if err != nil {
		panic(err)
	}
	return statements
}
