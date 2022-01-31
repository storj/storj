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

    public setMockAccessGrantsPage(mockAccessGrantsPage: AccessGrantsPage): void {
        this.mockAccessGrantsPage = mockAccessGrantsPage;
    }

    get(_projectId: string, _cursor: AccessGrantCursor): Promise<AccessGrantsPage> {
        return Promise.resolve(this.mockAccessGrantsPage);
    }

    create(_projectId: string, _name: string): Promise<AccessGrant> {
        return Promise.resolve(new AccessGrant('testId', 'testName', this.date, 'testKey'));
    }

    delete(_ids: string[]): Promise<void> {
        return Promise.resolve();
    }

    deleteByNameAndProjectID(_name: string, _projectID: string): Promise<void> {
        return Promise.resolve();
    }

    getGatewayCredentials(_accessGrant: string, _optionalURL?: string): Promise<GatewayCredentials> {
        return Promise.resolve(new GatewayCredentials('testCredId', new Date(), 'testAccessKeyId', 'testSecret', 'testEndpoint'));
    }
}
