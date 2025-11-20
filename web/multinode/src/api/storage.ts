// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { DiskSpace, DiskSpaceUsage, Stamp } from '@/storage';

/**
 * Client for storage controller of MND api.
 */
export class StorageClient extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/storage';

    /**
     * Returns storage usage information for selected node and satellite if any.
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
    public async usage(satelliteId: string | null, nodeId: string | null): Promise<DiskSpaceUsage> {
        let path = `${this.ROOT_PATH}`;

        if (satelliteId) {
            path += `/satellites/${satelliteId}`;
        }

        path += '/usage';

        if (nodeId) {
            path += `/${nodeId}`;
        }

        const now = new Date();
        const year = now.getUTCFullYear();
        const month = now.getUTCMonth() + 1;
        const period = `${year}-${month > 9 ? month : `0${month}`}`;

        path += `?period=${period}`;

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const data = await response.json();
        const usage = data.stamps || [];

        return new DiskSpaceUsage(
            usage.map(stamp => new Stamp(stamp.atRestTotal, stamp.atRestTotalBytes, new Date(stamp.intervalStart))),
            data.summary,
            data.summaryBytes,
        );
    }

    /**
     * Returns disk space information for selected node if selected.
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
    public async diskSpace(nodeId: string | null): Promise<DiskSpace> {
        let path = `${this.ROOT_PATH}/disk-space`;

        if (nodeId) {
            path += `/${nodeId}`;
        }

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const diskSpace = await response.json();

        return new DiskSpace(
            diskSpace.allocated,
            diskSpace.used,
            diskSpace.usedPieces,
            diskSpace.usedReclaimable,
            diskSpace.usedTrash,
            diskSpace.free,
            diskSpace.available,
            diskSpace.overused,
        );
    }
}
