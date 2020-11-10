// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { Dashboard, Satellite, Satellites } from '@/storagenode/sno/sno';

/**
 * SNOService is used to store and handle node information.
 * SNOService exposes a business logic related to node.
 */
export class StorageNodeService {
    private readonly node: StorageNodeApi;

    public constructor(api: StorageNodeApi) {
        this.node = api;
    }

    /**
     * Gets dashboard data from server.
     */
    public async dashboard(): Promise<Dashboard> {
        return await this.node.dashboard();
    }

    /**
     * Gets satellite data from server.
     * @param id - satellite id
     */
    public async satellite(id: string): Promise<Satellite> {
        return await this.node.satellite(id);
    }

    /**
     * Gets all satellites data from server.
     */
    public async satellites(): Promise<Satellites> {
        return await this.node.satellites();
    }
}
