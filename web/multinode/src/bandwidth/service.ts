// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { BandwidthClient } from '@/api/bandwidth';
import { BandwidthTraffic } from '@/bandwidth/index';

/**
 * exposes all bandwidth related logic
 */
export class Bandwidth {
    private readonly bandwidth: BandwidthClient;

    public constructor(bandwidth: BandwidthClient) {
        this.bandwidth = bandwidth;
    }

    /**
     * returns bandwidth for selected satellite and node if any.
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
        return await this.bandwidth.fetch(satelliteId, nodeId);
    }
}
