// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccessGrant,
    AccessGrantCursor,
    AccessGrantsApi,
    AccessGrantsPage,
    GatewayCredentials,
} from '@/types/accessGrants';

/**
 * Mock for AccessGrantsApi
 */
export class AccessGrantsMock implements AccessGrantsApi {
    private readonly date = new Date(0);
    private mockAccessGrantsPage: AccessGrantsPage;

    public setMockApiKeysPage(mockAccessGrantsPage: AccessGrantsPage): void {
        this.mockAccessGrantsPage = mockAccessGrantsPage;
    }

    get(projectId: string, cursor: AccessGrantCursor): Promise<AccessGrantsPage> {
        return Promise.resolve(this.mockAccessGrantsPage);
    }

    create(projectId: string, name: string): Promise<AccessGrant> {
        return Promise.resolve(new AccessGrant('testId', 'testName', this.date, 'testKey'));
    }

    delete(ids: string[]): Promise<void> {
        return Promise.resolve();
    }

    getGatewayCredentials(accessGrant: string): Promise<GatewayCredentials> {
        return Promise.resolve(new GatewayCredentials('testCredId', new Date(), 'testAccessKeyId', 'testSecret', 'testEndpoint'));
    }
}
