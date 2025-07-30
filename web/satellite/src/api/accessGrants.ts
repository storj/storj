// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccessGrant,
    AccessGrantCursor,
    AccessGrantsApi,
    AccessGrantsPage,
    EdgeCredentials,
} from '@/types/accessGrants';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

/**
 * AccessGrantsHttpApi is a http implementation of Access Grants API.
 * Exposes all access grants-related functionality
 */
export class AccessGrantsHttpApi implements AccessGrantsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/api-keys';

    /**
     * Fetch access grants.
     *
     * @returns AccessGrantsPage
     * @throws Error
     */
    public async get(projectId: string, cursor: AccessGrantCursor): Promise<AccessGrantsPage> {
        const path = `${this.ROOT_PATH}/list-paged?projectID=${projectId}&search=${cursor.search}&limit=${cursor.limit}&page=${cursor.page}&order=${cursor.order}&orderDirection=${cursor.orderDirection}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get API keys',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const apiKeys = await response.json();

        return this.getAccessGrantsPage(apiKeys);
    }

    /**
     * Used to create access grant.
     *
     * @param projectId - stores current project id
     * @param name - name of access grant that will be created
     * @param csrfProtectionToken - CSRF token
     * @returns AccessGrant
     * @throws Error
     */
    public async create(projectId: string, name: string, csrfProtectionToken: string): Promise<AccessGrant> {
        const path = `${this.ROOT_PATH}/create/${projectId}`;
        const response = await this.client.post(path, name, { csrfProtectionToken });

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not create new access grant',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        const info = result.keyInfo;
        const secret: string = result.key;

        return new AccessGrant(info.id, info.name, info.createdAt, secret);
    }

    /**
     * Used to delete access grant.
     *
     * @param ids - ids of access grants that will be deleted
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async delete(ids: string[], csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/delete-by-ids`;
        const response = await this.client.delete(path, JSON.stringify({ ids }), { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not delete access grants',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Fetch all API key names.
     *
     * @returns string[]
     * @throws Error
     */
    public async getAllAPIKeyNames(projectId: string): Promise<string[]> {
        const path = `${this.ROOT_PATH}/api-key-names?projectID=${projectId}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get access grant names',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return result ? result : [];
    }

    /**
     * Used to delete access grant by name and project ID.
     *
     * @param name - name of the access grant that will be deleted
     * @param projectID - id of the project where access grant was created
     * @param csrfProtectionToken - CSRF token
     * @throws Error
     */
    public async deleteByNameAndProjectID(name: string, projectID: string, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/delete-by-name?name=${name}&publicID=${projectID}`;
        const response = await this.client.delete(path, null, { csrfProtectionToken });

        if (response.ok || response.status === 204) {
            return;
        }

        throw new APIError({
            status: response.status,
            message: 'Can not delete access grant',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Used to get gateway credentials using access grant.
     *
     * @param accessGrant - generated access grant
     * @param requestURL - URL to which gateway credential requests are sent
     * @param isPublic - optional status
     * @throws Error
     */
    public async getGatewayCredentials(accessGrant: string, requestURL: string, isPublic = false): Promise<EdgeCredentials> {
        const path = `${requestURL}/v1/access`;
        const body = {
            access_grant: accessGrant,
            public: isPublic,
        };
        const response = await this.client.post(path, JSON.stringify(body));
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get gateway credentials',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        let freeTierRestrictedExpiration: Date | null = null;
        if (result.freeTierRestrictedExpiration) {
            freeTierRestrictedExpiration = new Date(result.freeTierRestrictedExpiration);
        }

        return new EdgeCredentials(
            result.id,
            new Date(result.created_at),
            result.access_key_id,
            result.secret_key,
            result.endpoint,
            freeTierRestrictedExpiration,
        );
    }

    /**
     * Method for mapping access grants page from json to AccessGrantsPage type.
     *
     * @param page anonymous object from json
     */
    private getAccessGrantsPage(page: any): AccessGrantsPage { // eslint-disable-line @typescript-eslint/no-explicit-any
        if (!(page && page.apiKeys)) {
            return new AccessGrantsPage();
        }

        const accessGrantsPage: AccessGrantsPage = new AccessGrantsPage();

        accessGrantsPage.accessGrants = page.apiKeys.map(key => new AccessGrant(
            key.id,
            key.name,
            new Date(key.createdAt),
            '',
            key.creatorEmail,
        ));

        accessGrantsPage.search = page.search;
        accessGrantsPage.limit = page.limit;
        accessGrantsPage.order = page.order;
        accessGrantsPage.orderDirection = page.orderDirection;
        accessGrantsPage.pageCount = page.pageCount;
        accessGrantsPage.currentPage = page.currentPage;
        accessGrantsPage.totalCount = page.totalCount;

        return accessGrantsPage;
    }
}
