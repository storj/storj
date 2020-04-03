// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all credits-related functionality.
 */
export interface CreditsApi {
    get(): Promise<CreditUsage>;
}

/**
 * CreditUsage stores information about users credits.
 * Tardigrade related logic
 */
export class CreditUsage {
    public referred: number;
    public usedCredits: number;
    public availableCredits: number;

    constructor(referred: number = 0, usedCredits: number = 0, availableCredits: number = 0) {
        this.referred = referred;
        this.usedCredits = usedCredits;
        this.availableCredits = availableCredits;
    }
}
