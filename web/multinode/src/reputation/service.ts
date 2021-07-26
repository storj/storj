// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ReputationClient } from '@/api/reputation';
import { Stats } from '@/reputation/index';

/**
 * ReputationService exposes all reputation related logic.
 */
export class ReputationService {
    private readonly reputation: ReputationClient;

    public constructor(reputation: ReputationClient) {
        this.reputation = reputation;
    }

    /**
     * stats handles retrieval of a node reputation for particular satellite.
     * @param satelliteId - id of satellite.
     */
    public async stats(satelliteId: string): Promise<Stats[]> {
        return await this.reputation.stats(satelliteId);
    }
}
