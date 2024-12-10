// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jmespath/go-jmespath"
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdSearch struct {
	access *AccessOptions

	rawPrefix  string
	matchFile  string
	rawMatch   string
	filter     string
	projection string

	bucket string
	prefix string
	match  map[string]interface{}
}

func newCmdSearch() *cmdSearch {
	return &cmdSearch{
		access: newAccessOptions(),
	}
}

func (c *cmdSearch) Setup(params clingy.Parameters) {
	c.access.Setup(params)

	c.rawMatch = params.Flag("match", "JSON metadata object to match", "", clingy.Short('m')).(string)
	c.matchFile = params.Flag("match-file", "File containing JSON metadata to match", "", clingy.Short('M')).(string)
	c.filter = params.Flag("filter", "JMESPath filter expression", "", clingy.Short('f')).(string)
	c.projection = params.Flag("projection", "JMESPath projection expression", "", clingy.Short('p')).(string)
	c.rawPrefix = params.Arg("prefix", "Object key prefix (sj://BUCKET[/PREFIX])").(string)
}

func (c *cmdSearch) Validate() (err error) {
	err = c.access.Validate()
	if err != nil {
		return err
	}

	loc, err := ulloc.Parse(c.rawPrefix)
	if err != nil {
		return fmt.Errorf("invalid location '%s': %w", c.rawPrefix, err)
	}

	var ok bool
	c.bucket, c.prefix, ok = loc.RemoteParts()
	if !ok {
		return fmt.Errorf("invalid location '%s': must be remote", c.rawPrefix)
	}

	if c.filter != "" {
		_, err = jmespath.Compile(c.filter)
		if err != nil {
			return fmt.Errorf("invalid filter expression: %w", err)
		}
	}

	if c.projection != "" {
		_, err = jmespath.Compile(c.projection)
		if err != nil {
			return fmt.Errorf("invalid projection expression: %w", err)
		}
	}

	return nil
}

func (c *cmdSearch) Execute(ctx context.Context) (err error) {
	err = c.Validate()
	if err != nil {
		return err
	}

	err = c.setMatch()
	if err != nil {
		return err
	}

	client := newMetaSearchClient(c.access)
	pageToken := ""
	fmt.Print("[")
	n := 0
	for i := 0; i == 0 || pageToken != ""; i++ {
		page, err := client.SearchMetadata(ctx, c.bucket, c.prefix, c.match, c.filter, c.projection, pageToken)
		if err != nil {
			return fmt.Errorf("error performing metadata search: %w", err)
		}

		for _, meta := range page.Results {
			formattedMeta, err := json.MarshalIndent(meta, "  ", "  ")
			if err != nil {
				return fmt.Errorf("cannot format metadata: %w", err)
			}
			if n > 0 {
				fmt.Print(",\n  ", string(formattedMeta))
			} else {
				fmt.Print("\n  ", string(formattedMeta))
			}
			n++
		}

		pageToken = page.PageToken
	}
	fmt.Println("\n]")

	return nil
}

func (c *cmdSearch) setMatch() (err error) {
	var match []byte
	if c.matchFile == "-" {
		match, err = io.ReadAll(os.Stdin)
	} else if c.matchFile != "" {
		match, err = os.ReadFile(c.matchFile)
	} else {
		match = []byte(c.rawMatch)
	}

	if err != nil {
		return fmt.Errorf("error reading match condition: %w", err)
	}

	if len(match) == 0 {
		return nil
	}

	err = json.Unmarshal(match, &c.match)
	if err != nil {
		return fmt.Errorf("invalid match condition: %w", err)
	}

	return nil
}
