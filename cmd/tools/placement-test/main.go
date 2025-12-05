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
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

var (
	rootCmd = &cobra.Command{
		Use:   "placement-test <countrycode:...,lastipport:...,lastnet:...,tag:signer/key/value,tag:signer/key/value...>",
		Short: "Test placement settings",
		Long: `"This command helps testing placement configuration.

You can define a custom node with attributes, and all available placement configuration will be tested against the node.

Supported node attributes:
  * countrycode
  * lastipport
  * lastnet
  * tag (value should be in the form of signer/key/value)

EXAMPLES:

placement-test --placement '10:country("GB");12:country("DE")' countrycode=11

placement-test --placement /tmp/proposal.txt countrycode=US,tag=12Q8q2PofHPwycSwAVCpjNxxzWiDJhi8UV4ceZBo4hmNARpYcR7/soc2/true

Where /tmp/proposal.txt contains definitions, for example:
10:tag("12Q8q2PofHPwycSwAVCpjNxxzWiDJhi8UV4ceZBo4hmNARpYcR7","selected",notEmpty());
1:country("EU") && exclude(placement(10)) && annotation("location","eu-1");
2:country("EEA") && exclude(placement(10)) && annotation("location","eea-1");
3:country("US") && exclude(placement(10)) && annotation("location","us-1");
4:country("DE") && exclude(placement(10)) && annotation("location","de-1");
6:country("*","!BY", "!RU", "!NONE") && exclude(placement(10)) && annotation("location","custom-1")
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return testPlacement(ctx, args[0])
		},
	}

	config Config
)

func testPlacement(ctx context.Context, fakeNode string) error {
	node := &nodeselection.SelectedNode{}
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

	placement, err := config.Placement.Parse(nil, nil)
	if err != nil {
		return errs.Wrap(err)
	}

	fmt.Println("Node:")
	jsonNode, err := json.MarshalIndent(node, "  ", "   ")
	if err != nil {
		return errs.Wrap(err)
	}

	fmt.Println(string(jsonNode))

	for _, placementNum := range placement.SupportedPlacements() {
		fmt.Printf("\n--------- Evaluating placement rule %d ---------\n", placementNum)
		filter, _ := placement.CreateFilters(placementNum)

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
	Placement nodeselection.ConfigurablePlacementRule `help:"detailed placement rules in the form 'id:definition;id:definition;...' where id is a 16 bytes integer (use >10 for backward compatibility), definition is a combination of the following functions:country(2 letter country codes,...), tag(nodeId, key, bytes(value)) all(...,...)."`
}

func init() {
	process.Bind(rootCmd, &config)
}

func main() {
	logger, _, _ := process.NewLogger("placement-test")
	zap.ReplaceGlobals(logger)

	process.ExecWithCustomOptions(rootCmd, process.ExecOptions{
		LoadConfig: func(cmd *cobra.Command, vip *viper.Viper) error {
			return nil
		},
		InitTracing: false,
		LoggerFactory: func(logger *zap.Logger) *zap.Logger {
			newLogger, level, err := process.NewLogger("placement-test")
			if err != nil {
				panic(err)
			}
			level.SetLevel(zap.WarnLevel)
			return newLogger
		},
	})
}
