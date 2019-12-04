// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ApiKey, ApiKeyCursor, ApiKeysApi, ApiKeysPage } from '@/types/apiKeys';

/**
 * Mock for ApiKeysApi
 */
export class ApiKeysMock implements ApiKeysApi {
    private mockApiKeysPage: ApiKeysPage;

    public setMockApiKeysPage(mockApiKeysPage: ApiKeysPage): void {
        this.mockApiKeysPage = mockApiKeysPage;
    }

    get(projectId: string, cursor: ApiKeyCursor): Promise<ApiKeysPage> {
        return Promise.resolve(this.mockApiKeysPage);
    }

    create(projectId: string, name: string): Promise<ApiKey> {
        throw new Error('Method not implemented');
    }

    delete(ids: string[]): Promise<void> {
        throw new Error('Method not implemented');
    }
}
