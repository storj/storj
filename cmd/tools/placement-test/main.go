// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"github.com/zeebo/structs"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/private/process"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

var (
	rootCmd = &cobra.Command{
		Use:   "placement-test <countrycode:...,lastipport:...,lastnet:...,tag:signer/key/value,tag:signer/key/value...>",
		Short: "Test placement settings",
		Long:  "",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return testPlacement(ctx, args[0])
		},
	}

	config Config
)

func testPlacement(ctx context.Context, fakeNode string) error {
	node := &nodeselection.SelectedNode{}
	values := map[string]interface{}{}
	for _, part := range strings.Split(fakeNode, ",") {
		kv := strings.SplitN(part, "=", 2)
		switch strings.ToLower(kv[0]) {
		case "countrycode":
			node.CountryCode = location.ToCountryCode(kv[1])
		case "lastipport":
			node.LastIPPort = kv[1]
		case "lastnet":
			node.LastNet = kv[1]
		case "tag":
			tkv := strings.SplitN(kv[1], "/", 3)
			signer, err := storj.NodeIDFromString(tkv[0])
			if err != nil {
				return err
			}
			node.Tags = append(node.Tags, nodeselection.NodeTag{
				Name:     tkv[1],
				Value:    []byte(tkv[2]),
				Signer:   signer,
				SignedAt: time.Now(),
				NodeID:   node.ID,
			})
		default:
			panic("Unsupported field of SelectedNode: " + kv[0])
		}

	}
	decodeResult := structs.Decode(values, &node)
	if decodeResult.Error != nil {
		return decodeResult.Error
	}

	placement, err := config.Placement.Parse()
	if err != nil {
		return errs.Wrap(err)
	}

	fmt.Println("Node:")
	jsonNode, err := json.MarshalIndent(node, "  ", "   ")
	if err != nil {
		return errs.Wrap(err)
	}

	fmt.Println(string(jsonNode))

	for _, placementNum := range placement.ConfiguredPlacements() {
		fmt.Printf("\n--------- Evaluating placement rule %d ---------\n", placementNum)
		filter := placement.CreateFilters(placementNum)

		fmt.Printf("Placement: %s\n", filter)
		result := filter.Match(node)
		fmt.Println("MATCH:    ", result)
		fmt.Println("Annotations: ")
		if annotated, ok := filter.(nodeselection.NodeFilterWithAnnotation); ok {
			fmt.Println("   location:", annotated.GetAnnotation("location"))
			fmt.Println("   "+nodeselection.AutoExcludeSubnet+":", annotated.GetAnnotation(nodeselection.AutoExcludeSubnet))
		} else {
			fmt.Println("    no annotation presents")
		}
	}
	return nil
}

// Config contains configuration of placement.
type Config struct {
	Placement overlay.ConfigurablePlacementRule `help:"detailed placement rules in the form 'id:definition;id:definition;...' where id is a 16 bytes integer (use >10 for backward compatibility), definition is a combination of the following functions:country(2 letter country codes,...), tag(nodeId, key, bytes(value)) all(...,...)."`
}

func init() {
	process.Bind(rootCmd, &config)
}

func main() {
	process.Exec(rootCmd)
}
