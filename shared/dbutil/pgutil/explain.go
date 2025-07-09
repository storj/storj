// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"context"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/storj/shared/tagsql"
)

// Explanation contains the result of a EXPLAIN.
type Explanation struct {
	Entries []ExplanationEntry
}

// ExplanationEntry is a single attribute of EXPLAIN query.
type ExplanationEntry struct {
	Key   string
	Value string
}

// Add adds a new entry to an Explanation.
func (e *Explanation) Add(key, value string) {
	e.Entries = append(e.Entries, ExplanationEntry{
		Key:   key,
		Value: value,
	})
}

// Select returns a new explanation containing only the specific keys.
func (e *Explanation) Select(keys ...string) Explanation {
	return Explanation{
		Entries: slices.DeleteFunc(slices.Clone(e.Entries),
			func(e ExplanationEntry) bool {
				return !slices.Contains(keys, e.Key)
			}),
	}
}

// Without returns a new explanation without the specified keys.
func (e *Explanation) Without(keys ...string) Explanation {
	return Explanation{
		Entries: slices.DeleteFunc(slices.Clone(e.Entries),
			func(e ExplanationEntry) bool {
				return slices.Contains(keys, e.Key)
			}),
	}
}

// String formats the explanation as a string.
func (e Explanation) String() string {
	var b strings.Builder
	for _, e := range e.Entries {
		_, _ = fmt.Fprintf(&b, "%v: %v\n", e.Key, e.Value)
	}
	return b.String()
}

// Explain explains the query.
func Explain(ctx context.Context, db tagsql.DB, query string, args ...any) (_ Explanation, err error) {
	inlinedQuery, err := RoughInlinePlaceholders(query, args...)
	if err != nil {
		return Explanation{}, fmt.Errorf("failed to inline arguments: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Explanation{}, fmt.Errorf("failed to start a transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var result Explanation

	inlinedQuery = "EXPLAIN ANALYZE " + inlinedQuery

	rows, err := tx.QueryContext(ctx, inlinedQuery)
	if err != nil {
		return Explanation{}, fmt.Errorf("explain failed %q: %w", inlinedQuery, err)
	}
	defer func() { err = errs.Combine(rows.Err(), rows.Close(), err) }()

	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return result, err
		}
		if row == "" {
			break
		}

		key, value, _ := strings.Cut(row, ":")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		result.Add(key, value)
	}

	plan := ""
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			return result, err
		}
		plan += "\n"
		plan += row
	}
	if plan != "" {
		result.Add("plan", plan)
	}

	return result, err
}

// RoughInlinePlaceholders does a very rough replacement of $N arguments.
// It does not properly parse the SQL query.
func RoughInlinePlaceholders(query string, args ...any) (string, error) {
	rx := regexp.MustCompile(`\$[0-9]+`)

	var errs errs.Group

	s := rx.ReplaceAllStringFunc(query, func(arg string) string {
		idx, err := strconv.Atoi(strings.TrimPrefix(arg, "$"))
		if err != nil {
			errs.Add(fmt.Errorf("failed to convert %q: %w", arg, err))
			return arg
		}
		idx--

		if idx < 0 || idx >= len(args) {
			errs.Add(fmt.Errorf("argument missing %v, but len(args) = %v", arg, len(args)))
			return arg
		}

		r, err := formatPostgresArgument(args[idx])
		if err != nil {
			errs.Add(fmt.Errorf("unable to convert %v: %w", arg, err))
			return arg
		}

		return r
	})

	return s, errs.Err()
}

func formatPostgresArgument(v any) (string, error) {
	value, err := driver.DefaultParameterConverter.ConvertValue(v)
	if err != nil {
		return "", fmt.Errorf("type %T: %w", v, err)
	}

	switch v := value.(type) {
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case []byte:
		return `'\x` + hex.EncodeToString(v) + `'`, nil
	case string:
		return `'` + strings.ReplaceAll(v, "'", "''") + `'`, nil
	case time.Time:
		return v.Truncate(time.Microsecond).Format("'2006-01-02 15:04:05.999999999Z07:00:00'"), nil
	case nil:
		return "NULL", nil
	default:
		return "", fmt.Errorf("unhandled type %T", v)
	}
}
