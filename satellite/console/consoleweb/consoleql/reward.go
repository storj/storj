// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
)

const (
	// RewardType is a graphql type for reward
	RewardType = "reward"
	// FieldAwardCreditInCent is a field name for award credit amount for referrers
	FieldAwardCreditInCent = "awardCreditInCent"
	// FieldInviteeCreditInCents is a field name for credit amount rewarded to invitees
	FieldInviteeCreditInCents = "inviteeCreditInCents"
	// FieldRedeemableCap is a field name for the total redeemable amount of the reward offer
	FieldRedeemableCap = "redeemableCap"
	// FieldAwardCreditDurationDays is a field name for the valid time frame of current award credit
	FieldAwardCreditDurationDays = "awardCreditDurationDays"
	// FieldInviteeCreditDurationDays is a field name for the valid time frame of current invitee credit
	FieldInviteeCreditDurationDays = "inviteeCreditDurationDays"
	// FieldExpiresAt is a field name for the expiration time of a reward offer
	FieldExpiresAt = "expiresAt"
	// FieldType is a field name for the type of reward offers
	FieldType = "type"
	// FieldStatus is a field name for the status of reward offers
	FieldStatus = "status"
)

func graphqlReward() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: RewardType,
		Fields: graphql.Fields{
			FieldID: &graphql.Field{
				Type: graphql.Int,
			},
			FieldAwardCreditInCent: &graphql.Field{
				Type: graphql.Int,
			},
			FieldInviteeCreditInCents: &graphql.Field{
				Type: graphql.Int,
			},
			FieldRedeemableCap: &graphql.Field{
				Type: graphql.Int,
			},
			FieldAwardCreditDurationDays: &graphql.Field{
				Type: graphql.Int,
			},
			FieldInviteeCreditDurationDays: &graphql.Field{
				Type: graphql.Int,
			},
			FieldType: &graphql.Field{
				Type: graphql.Int,
			},
			FieldStatus: &graphql.Field{
				Type: graphql.Int,
			},
			FieldExpiresAt: &graphql.Field{
				Type: graphql.String,
			},
		},
	})
}
