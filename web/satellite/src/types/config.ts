// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { FrontendConfig } from '@/types/config.gen';
export * from '@/types/config.gen';

/**
 * Exposes functionality related to retrieving the frontend config.
 */
export interface FrontendConfigApi {
    /**
     * Returns the frontend config.
     *
     * @throws Error
     */
    get(): Promise<FrontendConfig>;
}
