// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"regexp"
	"strings"
)

func main() {
	{
		raw, err := os.ReadFile("attribution_all_value_psql.sql")
		if err != nil {
			panic(err)
		}

		query := spannerify(string(raw))
		query = strings.ReplaceAll(query, "$1", "@start")
		query = strings.ReplaceAll(query, "$2", "@end")

		err = os.WriteFile("attribution_all_value_spanner.sql", []byte(query), 0644)
		if err != nil {
			panic(err)
		}
	}

	{
		raw, err := os.ReadFile("attribution_value_psql.sql")
		if err != nil {
			panic(err)
		}

		query := spannerify(string(raw))
		query = strings.ReplaceAll(query, "$1", "@user_agent")
		query = strings.ReplaceAll(query, "$2", "@start")
		query = strings.ReplaceAll(query, "$3", "@end")

		err = os.WriteFile("attribution_value_spanner.sql", []byte(query), 0644)
		if err != nil {
			panic(err)
		}
	}
}

func spannerify(query string) string {

	query = strings.ReplaceAll(query, "UNION", "UNION ALL")
	query = convertTimestampTrunc(query)
	query = strings.ReplaceAll(query, "SUM(settled)::integer", "CAST(SUM(bbr.settled) as INT64)")
	return query
}

func convertTimestampTrunc(src string) string {
	pattern := `date_trunc\('hour',\s*(\w+\.\w+)\)`
	replacement := `timestamp_trunc($1, hour, "UTC")`

	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(src, replacement)
}
