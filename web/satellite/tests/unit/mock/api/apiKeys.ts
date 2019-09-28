// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ApiKey, ApiKeyCursor, ApiKeysApi, ApiKeysPage } from '@/types/apiKeys';

/**
 * Mock for ApiKeysApi
 */
export class ApiKeysMock implements ApiKeysApi {
    get(projectId: string, cursor: ApiKeyCursor): Promise<ApiKeysPage> {
        throw new Error('Method not implemented');
    }

    create(projectId: string, name: string): Promise<ApiKey> {
        throw new Error('Method not implemented');
    }

    delete(ids: string[]): Promise<void> {
        throw new Error('Method not implemented');
    }
}
