// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { SortDirection } from '@/types/common';

export type OnHeaderClickCallback = (sortBy: ApiKeyOrderBy, sortDirection: SortDirection) => Promise<void>;

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
    get(projectId: string, cursor: ApiKeyCursor): Promise<ApiKeysPage>;

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
    delete(ids: string[]): Promise<void>;
}

export enum ApiKeyOrderBy {
    NAME = 1,
    EMAIL,
    CREATED_AT,
}

// ApiKeyCursor is a type, used to describe paged api keys list
export class ApiKeyCursor {
    public constructor(
        public search: string = '',
        public limit: number = 6,
        public page: number = 1,
        public order: ApiKeyOrderBy = ApiKeyOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING) {
    }
}

// ApiKeysPage is a type, used to describe paged api keys list
export class ApiKeysPage {
    public constructor(
        public apiKeys: ApiKey[] = [],
        public search: string = '',
        public order: ApiKeyOrderBy = ApiKeyOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0) {
    }
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
