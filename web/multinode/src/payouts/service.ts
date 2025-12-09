// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { PayoutsClient } from '@/api/payouts';
import { Expectation, HeldAmountSummary, PayoutsSummary, Paystub } from '@/payouts/index';

/**
 * Exposes all payouts related logic
 */
export class Payouts {
    private readonly payouts: PayoutsClient;

    public constructor(payouts: PayoutsClient) {
        this.payouts = payouts;
    }

    /**
     * Fetches payouts summary information.
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
     * Fetches payouts expectation such as estimated current month payout and undistributed payout.
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
    public async expectations(nodeId?: string): Promise<Expectation> {
        return await this.payouts.expectations(nodeId);
    }

    /**
     * Fetches total paystub of given node, period and satellite.
     *
     * @param satelliteId
     * @param period
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
    public async paystub(satelliteId: string | null, period: string | null, nodeId: string): Promise<Paystub> {
        return await this.payouts.paystub(satelliteId, period, nodeId);
    }

    /**
     * Fetches held history of given node.
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
    public async heldHistory(nodeId: string): Promise<HeldAmountSummary[]> {
        return await this.payouts.heldHistory(nodeId);
    }
}
