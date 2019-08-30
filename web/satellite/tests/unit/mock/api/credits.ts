// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { CreditsApi, CreditUsage } from '@/types/credits';

/**
 * Mock for CreditsApi
 */
export class CreditsApiMock implements CreditsApi {
    private mockCredits: CreditUsage;

    get(): Promise<CreditUsage> {
        return Promise.resolve(this.mockCredits);
    }

    public setMockCredits(mockCredits: CreditUsage): void {
        this.mockCredits = mockCredits;
    }
}
