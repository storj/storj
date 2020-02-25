// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { HttpClient } from '@/utils/httpClient';

/**
 * ReferralHttpApi is a console Referral API.
 * Exposes all referral-related functionality
 */
export class ReferralHttpApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/referrals';

    /**
     * Used to get referral tokens.
     *
     * @throws Error
     */
    public async getTokens(): Promise<string[]> {
        const path = `${this.ROOT_PATH}/tokens`;
        const response = await this.http.get(path);

        if (response.ok) {
            return await response.json();
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not get referral tokens');
    }
}
