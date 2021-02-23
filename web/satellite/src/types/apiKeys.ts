// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { SortDirection } from '@/types/common';

export type OnHeaderClickCallback = (sortBy: ApiKeyOrderBy, sortDirection: SortDirection) => Promise<void>;

/**
 * Exposes all apiKey-related functionality.
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

/**
 * Holds api keys sorting parameters.
 */
export enum ApiKeyOrderBy {
    NAME = 1,
    CREATED_AT,
}

/**
 * ApiKeyCursor is a type, used to describe paged api keys list.
  */
export class ApiKeyCursor {
    public constructor(
        public search: string = '',
        public limit: number = 6,
        public page: number = 1,
        public order: ApiKeyOrderBy = ApiKeyOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
    ) {}
}

/**
 * ApiKeysPage is a type, used to describe paged api keys list.
 */
export class ApiKeysPage {
    public constructor(
        public apiKeys: ApiKey[] = [],
        public search: string = '',
        public order: ApiKeyOrderBy = ApiKeyOrderBy.NAME,
        public orderDirection: SortDirection = SortDirection.ASCENDING,
        public limit: number = 6,
        public pageCount: number = 0,
        public currentPage: number = 1,
        public totalCount: number = 0,
    ) {}
}

/**
 * ApiKey class holds info for ApiKeys entity.
 */
export class ApiKey {
    public isSelected: boolean;

    constructor(
        public id: string = '',
        public name: string = '',
        public createdAt: Date = new Date(),
        public secret: string = '',
    ) {
        this.isSelected = false;
    }

    /**
     * Returns created date as a local date string.
     */
    public localDate(): string {
        return this.createdAt.toLocaleDateString();
    }
}
