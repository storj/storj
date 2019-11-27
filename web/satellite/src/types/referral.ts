// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all referral-related functionality
 */
export interface ReferralApi {
    /**
     * Get referral links for account
     *
     * @returns links
     * @throws Error
     */
    getLinks(): Promise<any>;
}
