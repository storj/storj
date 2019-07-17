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