// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { ABHitAction, ABTestApi, ABTestValues } from '@/types/abtesting';

/**
 * ABHttpApi is a console AB testing API.
 * Exposes all ab-testing related functionality
 */
export class ABMockApi implements ABTestApi {
    public async fetchABTestValues(): Promise<ABTestValues> {
        return new ABTestValues();
    }

    public async sendHit(_: ABHitAction): Promise<void> {
        return;
    }
}
