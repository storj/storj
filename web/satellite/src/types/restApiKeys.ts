// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all API Keys related functionality.
 */
export interface RestApiKeysApi {
    /**
     * Fetch apiKeys
     *
     * @returns RestApiKey[]
     * @throws Error
     */
    getAll(): Promise<RestApiKey[]>;

    /**
     * Create new apiKey
     *
     * @returns apiKey string
     * @throws Error
     */
    create(name: string, expiration: number, csrfProtectionToken: string): Promise<string>;

    /**
     * Delete existing apiKeys
     *
     * @returns null
     * @throws Error
     */
    delete(ids: string[], csrfProtectionToken: string): Promise<void>;
}

/**
 * AccessGrant class holds info for Access Grant entity.
 */
export class RestApiKey {
    constructor(
        public id: string = '',
        public name: string = '',
        public createdAt: Date = new Date(),
        public expiresAt: Date | null = null,
    ) { }
}
