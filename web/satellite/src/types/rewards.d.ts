// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare type Reward = {
    id: number,
    awardCreditInCent?: number,
    inviteeCreditInCents: number,
    redeemableCap: number,
    awardCreditDurationDays?: number,
    inviteeCreditDurationDays: number,
    type: number,
    status: number,
    expiresAt: string,
}