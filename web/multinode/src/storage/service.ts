// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { StorageClient } from '@/api/storage';
import { DiskSpace, DiskSpaceUsage } from '@/storage';

/**
 * Exposes all bandwidth related logic
 */
export class StorageService {
    private readonly storage: StorageClient;

    public constructor(bandwidth: StorageClient) {
        this.storage = bandwidth;
    }

    /**
     * Returns storage usage for selected satellite and node if any.
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
        return await this.storage.usage(satelliteId, nodeId);
    }

    /**
     * Returns total storage usage for selected node if any.
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
        return await this.storage.diskSpace(nodeId);
    }
}
