// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/process"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/shared/nodetag"
)

var (
	rootCmd = &cobra.Command{
		Use:   "tag-signer",
		Short: "Sign key=value pairs with identity",
		Long: "Node tags are arbitrary key value pairs signed by an authority. If the public key is configured on " +
			"Satellite side, Satellite will check the signatures and save the tags, which can be used (for example)" +
			" during node selection. Storagenodes can be configured to send encoded node tags to the Satellite. " +
			"This utility helps creating/managing the values of this specific configuration value, which is encoded by default.",
	}

	signCmd = &cobra.Command{
		Use:   "sign <key=value> <key2=value> ...",
		Short: "Create signed tagset",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			encoded, err := signTags(ctx, config, args)
			if err != nil {
				return err
			}
			fmt.Println(encoded)
			return nil
		},
	}

	inspectCmd = &cobra.Command{
		Use:   "inspect <encoded string>",
		Short: "Print out the details from an encoded node set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return inspect(ctx, args[0])
		},
	}

	config Config
)

// Config contains configuration required for signing.
type Config struct {
	IdentityDir string `help:"location if the identity files" path:"true"`
	NodeID      string `help:"the ID of the node, which will used this tag "`
	Confirm     bool   `help:"enable comma in tag values" default:"false"`
}

func init() {
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(inspectCmd)
	process.Bind(signCmd, &config)
}

func signTags(ctx context.Context, cfg Config, tagPairs []string) (string, error) {

	if cfg.IdentityDir == "" {
		return "", errs.New("Please specify the identity, used as a signer with --identity-dir")
	}

	if cfg.NodeID == "" {
		return "", errs.New("Please specify the --node-id")
	}

	identityConfig := identity.Config{
		CertPath: filepath.Join(cfg.IdentityDir, "identity.cert"),
		KeyPath:  filepath.Join(cfg.IdentityDir, "identity.key"),
	}

	fullIdentity, err := identityConfig.Load()
	if err != nil {
		return "", err
	}

	signer := signing.SignerFromFullIdentity(fullIdentity)

	nodeID, err := storj.NodeIDFromString(cfg.NodeID)
	if err != nil {
		return "", errs.New("Wrong NodeID format: %v", err)
	}
	tagSet := &pb.NodeTagSet{
		NodeId:   nodeID.Bytes(),
		SignedAt: time.Now().Unix(),
	}

	tagSet.Tags, err = parseTagPairs(tagPairs, cfg.Confirm)
	if err != nil {
		return "", err
	}

	signedMessage, err := nodetag.Sign(ctx, tagSet, signer)
	if err != nil {
		return "", err
	}

	all := &pb.SignedNodeTagSets{
		Tags: []*pb.SignedNodeTagSet{
			signedMessage,
		},
	}

	raw, err := pb.Marshal(all)
	if err != nil {
		return "", errs.Wrap(err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func inspect(ctx context.Context, s string) error {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return errs.New("Input is not in base64 format")
	}

	sets := &pb.SignedNodeTagSets{}
	err = pb.Unmarshal(raw, sets)
	if err != nil {
		return errs.New("Input is not a protobuf encoded *pb.SignedNodeTagSets message")
	}

	for _, msg := range sets.Tags {

		signerNodeID, err := storj.NodeIDFromBytes(msg.SignerNodeId)
		if err != nil {
			return err
		}

		fmt.Println("Signer:            ", signerNodeID.String())
		fmt.Println("Signature:         ", hex.EncodeToString(msg.Signature))

		tags := &pb.NodeTagSet{}
		err = pb.Unmarshal(msg.SerializedTag, tags)
		if err != nil {
			return err
		}
		nodeID, err := storj.NodeIDFromBytes(tags.NodeId)
		if err != nil {
			return err
		}

		fmt.Println("SignedAt:          ", time.Unix(tags.SignedAt, 0).Format(time.RFC3339))
		fmt.Println("NodeID:            ", nodeID.String())
		fmt.Println("Tags:")
		for _, tag := range tags.Tags {
			fmt.Printf("   %s=%s\n", tag.Name, string(tag.Value))
		}
		fmt.Println()
	}
	return nil
}

func parseTagPairs(tagPairs []string, allowCommaValues bool) ([]*pb.Tag, error) {
	tags := make([]*pb.Tag, 0, len(tagPairs))

	for _, tag := range tagPairs {
		tag = strings.TrimSpace(tag)
		if len(tag) == 0 {
			continue
		}

		if !allowCommaValues && strings.ContainsRune(tag, ',') {
			return nil, errs.New("multiple tags should be separated by spaces instead of commas, or specify --confirm to enable commas in tag values")
		}

		parts := strings.SplitN(tag, "=", 2)
		if len(parts) != 2 {
			return nil, errs.New("tags should be in KEY=VALUE format, but it was %s", tag)
		}
		tags = append(tags, &pb.Tag{
			Name:  parts[0],
			Value: []byte(parts[1]),
		})
	}

	return tags, nil
}

func main() {
	logger, _, _ := process.NewLogger("tag-signer")
	zap.ReplaceGlobals(logger)

	process.ExecWithCustomOptions(rootCmd, process.ExecOptions{
		LoadConfig: func(cmd *cobra.Command, vip *viper.Viper) error {
			return nil
		},
		InitTracing: false,
		LoggerFactory: func(logger *zap.Logger) *zap.Logger {
			newLogger, level, err := process.NewLogger("tag-signer")
			if err != nil {
				panic(err)
			}
			level.SetLevel(zap.WarnLevel)
			return newLogger
		},
	})
}
