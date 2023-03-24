// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { FrontendConfig, FrontendConfigApi } from '@/types/config';

/**
 * FrontendConfigHttpApi is a mock implementation of the frontend config API.
 */
export class FrontendConfigApiMock implements FrontendConfigApi {
    public async get(): Promise<FrontendConfig> {
        return new FrontendConfig();
    }
}
