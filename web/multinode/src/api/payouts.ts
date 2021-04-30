// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { NodePayoutsSummary, PayoutsSummary } from '@/payouts';

/**
 * client for nodes controller of MND api.
 */
export class PayoutsClient extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/payouts';

    /**
     * handles fetch of payouts summary information.
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
        let path = `${this.ROOT_PATH}/summary`;

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
}
