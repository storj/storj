// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { CreditsApi, CreditUsage } from '@/types/credits';

/**
 * CreditsApiGql is a graphql implementation of Credits API.
 * Exposes all credits-related functionality
 */
export class CreditsApiGql extends BaseGql implements CreditsApi {
    /**
     * Fetch CreditUsage
     *
     * @returns CreditUsage
     * @throws Error
     */
    public async get(): Promise<CreditUsage> {
        const query =
            `query {
                creditUsage {
                    referred,
                    usedCredit,
                    availableCredit,
                }
            }`;

        const response = await this.query(query);

        return this.fromJson(response.data.creditUsage);
    }

    /**
     * Method for mapping credit usage from json to CreditUsage type.
     *
     * @param jsonCredits anonymous object from json
     */
    private fromJson(jsonCredits): CreditUsage {
        return new CreditUsage(jsonCredits.referred, jsonCredits.usedCredit, jsonCredits.availableCredit);
    }
}
