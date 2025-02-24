// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';
import { RestApiKey, RestApiKeysApi } from '@/types/restApiKeys';

/**
 * ApiKeysHttpApi is a http implementation of API Keys API.
 */
export class ApiKeysHttpApi implements RestApiKeysApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/restkeys';

    /**
     * Fetch apiKeys
     *
     * @returns RestApiKey
     * @throws Error
     */
    public async getAll(): Promise<RestApiKey[]> {
        const response = await this.client.get(this.ROOT_PATH);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get API keys',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const keys = await response.json();
        return (keys ?? []).map((key: RestApiKey) => new RestApiKey(
            key.id,
            key.name,
            key.createdAt,
            key.expiresAt,
        ));
    }

    /**
     * Create new apiKey
     *
     * @returns apiKey string
     * @throws Error
     */
    public async create(name: string, expiration: number | null, csrfProtectionToken: string): Promise<string> {
        const path = this.ROOT_PATH;
        const response = await this.client.post(path, JSON.stringify({ name, expiration }), { csrfProtectionToken });

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not create new API Key',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return await response.json();
    }

    /**
     * Delete existing apiKeys
     *
     * @returns null
     * @throws Error
     */
    public async delete(ids: string[], csrfProtectionToken: string): Promise<void> {
        const path = this.ROOT_PATH;
        const response = await this.client.delete(path, JSON.stringify({ ids }), { csrfProtectionToken });

        if (response.ok) {
            return;
        }

        const result = await response.json();

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not delete API keys',
            requestID: response.headers.get('x-request-id'),
        });
    }
}
