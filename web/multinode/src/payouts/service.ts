// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { PayoutsClient } from '@/api/payouts';
import { Expectations, PayoutsSummary } from '@/payouts/index';

/**
 * exposes all payouts related logic
 */
export class Payouts {
    private readonly payouts: PayoutsClient;

    public constructor(payouts: PayoutsClient) {
        this.payouts = payouts;
    }

    /**
     * fetches of payouts summary information.
     *
     * @param satelliteId - satellite id.
     * @param period - selected period.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async summary(satelliteId: string | null, period: string | null): Promise<PayoutsSummary> {
        return await this.payouts.summary(satelliteId, period);
    }

    /**
     * fetches of payouts expectation such as estimated current month payout and undistributed payout.
     *
     * @param nodeId - node id.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async expectations(nodeId: string | null): Promise<Expectations> {
        return await this.payouts.expectations(nodeId);
    }
}
