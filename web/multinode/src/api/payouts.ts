// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { Expectations, NodePayoutsSummary, PayoutsSummary } from '@/payouts';

/**
 * client for nodes controller of MND api.
 */
export class PayoutsClient extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/payouts';

    /**
     * Handles fetch of payouts summary information.
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
        let path = `${this.ROOT_PATH}/`;

        if (satelliteId) {
            path += `/satellites/${satelliteId}`;
        }

        path += '/summaries';

        if (period) {
            path += `/${period}`;
        }

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return new PayoutsSummary(
            result.totalEarned,
            result.totalHeld,
            result.totalPaid,
            result.nodeSummary.map(item => new NodePayoutsSummary(
                item.nodeId,
                item.nodeName,
                item.held,
                item.paid,
            )),
        );
    }

    /**
     * Handles fetch of payouts expectation such as estimated current month payout and undistributed payout.
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
        let path = `${this.ROOT_PATH}/expectations`;

        if (nodeId) {
            path += `/${nodeId}`;
        }

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return new Expectations(
            result.currentMonthEstimation,
            result.undistributed,
        );
    }
}
