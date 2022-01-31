// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { BandwidthRollup, BandwidthTraffic, Egress, Ingress } from '@/bandwidth';

/**
 * Client for bandwidth controller of MND api.
 */
export class BandwidthClient extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/bandwidth';

    /**
     * Returns bandwidth information for selected node and satellite if any.
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
    public async fetch(satelliteId: string | null, nodeId: string | null): Promise<BandwidthTraffic> {
        let path = `${this.ROOT_PATH}`;

        if (!satelliteId && !nodeId) {
            path += '/';
        }

        if (satelliteId) {
            path += `/satellites/${satelliteId}`;
        }

        if (nodeId) {
            path += `/${nodeId}`;
        }

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const traffic = await response.json();
        const daily = traffic.bandwidthDaily || [];

        return new BandwidthTraffic(
            daily.map(daily => new BandwidthRollup(
                new Egress(daily.egress.repair, daily.egress.audit, daily.egress.usage),
                new Ingress(daily.ingress.repair, daily.ingress.usage),
                daily.delete,
                new Date(daily.intervalStart),
            )),
            traffic.bandwidthSummary,
            traffic.egressSummary,
            traffic.ingressSummary,
        );
    }
}
