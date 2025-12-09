// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { Expectation, HeldAmountSummary, NodePayoutsSummary, PayoutsSummary, Paystub } from '@/payouts';

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
        let path = `${this.ROOT_PATH}`;

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
    public async expectations(nodeId?: string): Promise<Expectation> {
        let path = `${this.ROOT_PATH}/expectations`;

        if (nodeId) {
            path += `/${nodeId}`;
        }

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return new Expectation(
            result.currentMonthEstimation,
            result.undistributed,
        );
    }

    /**
     * Handles fetch of payouts paystub information for node.
     *
     * @param satelliteId - satellite id.
     * @param period - selected period.
     * @param nodeId
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
        let path = `${this.ROOT_PATH}`;

        if (satelliteId) {
            path += `/satellites/${satelliteId}`;
        }

        path += '/paystubs';

        if (period) {
            path += `/${period}`;
        }

        path += `/${nodeId}`;

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return new Paystub(
            result.usageAtRest,
            result.usageGet,
            result.usageGetRepair,
            result.usageGetAudit,
            result.compAtRest,
            result.compGet,
            result.compGetRepair,
            result.compGetAudit,
            result.held,
            result.paid,
            result.distributed,
        );
    }

    /**
     * Handles fetch of payouts paystub information for node.
     *
     * @param nodeId
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
        const path = `${this.ROOT_PATH}/held-amounts/${nodeId}`;

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const result = await response.json();

        return result.map(heldHistoryItem => new HeldAmountSummary(
            heldHistoryItem.satelliteAddress,
            heldHistoryItem.firstQuarter,
            heldHistoryItem.secondQuarter,
            heldHistoryItem.thirdQuarter,
            heldHistoryItem.periodCount,
        ));
    }
}
