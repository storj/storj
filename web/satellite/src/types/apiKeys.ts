// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all apiKey-related functionality
 */
export interface ApiKeysApi {
    /**
     * Fetch apiKeys
     *
     * @returns ApiKey[]
     * @throws Error
     */
    get(projectId: string): Promise<ApiKey[]>;
    /**
     * Create new apiKey
     *
     * @returns ApiKey
     * @throws Error
     */
    create(projectId: string, name: string): Promise<ApiKey>;
    /**
     * Delete existing apiKey
     *
     * @returns null
     * @throws Error
     */
    delete(ids: string[]): Promise<null>;
}

/**
 * ApiKey class holds info for ApiKeys entity.
 */
export class ApiKey {
    public id: string;
    public secret: string;
    public name: string;
    public createdAt: string;
    public isSelected: boolean = false;

    constructor(id: string, name: string, createdAt: string, secret: string) {
        this.id = id || '';
        this.name = name || '';
        this.createdAt = createdAt || '';
        this.secret = secret || '';

        this.isSelected = false;
    }

    public formattedName(): string {
        const name = this.name;

        if (name.length < 12) {
            return name;
        }

        return name.slice(0, 12) + '...';
    }

    public getDate(): string {
        if (!this.createdAt) {
            return '';
        }

        return new Date(this.createdAt).toLocaleDateString();
    }
}
