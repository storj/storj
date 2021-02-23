// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all referral-related functionality.
 */
export interface ReferralApi {
    /**
     * Get referral links for account.
     *
     * @returns links
     * @throws Error
     */
    getTokens(): Promise<string[]>;
}

/**
 * ReferralLink creates url from token.
 */
export class ReferralLink {
    public url: string = '';

    constructor(token: string = '') {
        this.url = `${location.host}/register?referralToken=${token}`;
    }
}
