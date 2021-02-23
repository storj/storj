// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ApiKey, ApiKeyCursor, ApiKeysApi, ApiKeysPage } from '@/types/apiKeys';

/**
 * ApiKeysApiGql is a graphql implementation of ApiKeys API.
 * Exposes all apiKey-related functionality
 */
export class ApiKeysApiGql extends BaseGql implements ApiKeysApi {
    /**
     * Fetch apiKeys.
     *
     * @returns ApiKey
     * @throws Error
     */
    public async get(projectId: string, cursor: ApiKeyCursor): Promise<ApiKeysPage> {
        const query =
            `query($projectId: String!, $limit: Int!, $search: String!, $page: Int!, $order: Int!, $orderDirection: Int!) {
                project (
                    id: $projectId,
                ) {
                    apiKeys (
                        cursor: {
                            limit: $limit,
                            search: $search,
                            page: $page,
                            order: $order,
                            orderDirection: $orderDirection
                        }
                    ) {
                        apiKeys {
                            id,
                            name,
                            createdAt
                        }
                        search,
                        limit,
                        order,
                        pageCount,
                        currentPage,
                        totalCount
                    }
                }
            }`;

        const variables = {
            projectId: projectId,
            limit: cursor.limit,
            search: cursor.search,
            page: cursor.page,
            order: cursor.order,
            orderDirection: cursor.orderDirection,
        };

        const response = await this.query(query, variables);

        return this.getApiKeysPage(response.data.project.apiKeys);
    }

    /**
     * Used to create apiKey.
     *
     * @param projectId - stores current project id
     * @param name - name of apiKey that will be created
     * @returns ApiKey
     * @throws Error
     */
    public async create(projectId: string, name: string): Promise<ApiKey> {
        const query =
            `mutation($projectId: String!, $name: String!) {
                createAPIKey(
                    projectID: $projectId,
                    name: $name
                ) {
                    key,
                    keyInfo {
                        id,
                        name,
                        createdAt
                    }
                }
            }`;

        const variables = {
            projectId,
            name,
        };

        const response = await this.mutate(query, variables);
        const key: any = response.data.createAPIKey.keyInfo;
        const secret: string = response.data.createAPIKey.key;

        return new ApiKey(key.id, key.name, key.createdAt, secret);
    }

    /**
     * Used to delete apiKey.
     *
     * @param ids - ids of apiKeys that will be deleted
     * @throws Error
     */
    public async delete(ids: string[]): Promise<void> {
        const query =
            `mutation($id: [String!]!) {
                deleteAPIKeys(id: $id) {
                    id
                }
            }`;

        const variables = {
            id: ids,
        };

        const response = await this.mutate(query, variables);

        return response.data.deleteAPIKeys;
    }

    /**
     * Method for mapping api keys page from json to ApiKeysPage type.
     *
     * @param page anonymous object from json
     */
    private getApiKeysPage(page: any): ApiKeysPage {
        if (!page) {
            return new ApiKeysPage();
        }

        const apiKeysPage: ApiKeysPage = new ApiKeysPage();

        apiKeysPage.apiKeys = page.apiKeys.map(key => new ApiKey(
            key.id,
            key.name,
            new Date(key.createdAt),
            '',
        ));

        apiKeysPage.search = page.search;
        apiKeysPage.limit = page.limit;
        apiKeysPage.order = page.order;
        apiKeysPage.orderDirection = page.orderDirection;
        apiKeysPage.pageCount = page.pageCount;
        apiKeysPage.currentPage = page.currentPage;
        apiKeysPage.totalCount = page.totalCount;

        return apiKeysPage;
    }
}
