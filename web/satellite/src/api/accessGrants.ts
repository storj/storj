// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { AccessGrant, AccessGrantCursor, AccessGrantsApi, AccessGrantsPage } from '@/types/accessGrants';

/**
 * AccessGrantsApiGql is a graphql implementation of Access Grants API.
 * Exposes all access grants-related functionality
 */
export class AccessGrantsApiGql extends BaseGql implements AccessGrantsApi {
    /**
     * Fetch access grants.
     *
     * @returns AccessGrantsPage
     * @throws Error
     */
    public async get(projectId: string, cursor: AccessGrantCursor): Promise<AccessGrantsPage> {
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

        return this.getAccessGrantsPage(response.data.project.apiKeys);
    }

    /**
     * Used to create access grant.
     *
     * @param projectId - stores current project id
     * @param name - name of access grant that will be created
     * @returns AccessGrant
     * @throws Error
     */
    public async create(projectId: string, name: string): Promise<AccessGrant> {
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

        return new AccessGrant(key.id, key.name, key.createdAt, secret);
    }

    /**
     * Used to delete access grant.
     *
     * @param ids - ids of access grants that will be deleted
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
     * Method for mapping access grants page from json to AccessGrantsPage type.
     *
     * @param page anonymous object from json
     */
    private getAccessGrantsPage(page: any): AccessGrantsPage {
        if (!page) {
            return new AccessGrantsPage();
        }

        const accessGrantsPage: AccessGrantsPage = new AccessGrantsPage();

        accessGrantsPage.accessGrants = page.apiKeys.map(key => new AccessGrant(
            key.id,
            key.name,
            new Date(key.createdAt),
            '',
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
