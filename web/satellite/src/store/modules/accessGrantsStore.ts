// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import {
    AccessGrant,
    AccessGrantCursor,
    AccessGrantsOrderBy,
    AccessGrantsPage,
    EdgeCredentials,
} from '@/types/accessGrants';
import { SortDirection } from '@/types/common';
import { AccessGrantsHttpApi } from '@/api/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

class AccessGrantsState {
    public allAGNames: string[] = [];
    public cursor: AccessGrantCursor = new AccessGrantCursor();
    public page: AccessGrantsPage = new AccessGrantsPage();
    public accessGrantsWebWorker: Worker | null = null;
}

export const useAccessGrantsStore = defineStore('accessGrants', () => {
    const api = new AccessGrantsHttpApi();

    const state = reactive<AccessGrantsState>(new AccessGrantsState());

    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();

    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    function setWorker(worker: Worker | null): void {
        state.accessGrantsWebWorker = worker;
    }

    async function getAllAGNames(projectID: string): Promise<void> {
        state.allAGNames = await api.getAllAPIKeyNames(projectID);
    }

    async function getAccessGrants(pageNumber: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<AccessGrantsPage> {
        state.cursor.page = pageNumber;
        state.cursor.limit = limit;

        const accessGrantsPage: AccessGrantsPage = await api.get(projectID, state.cursor);

        state.page = accessGrantsPage;

        return accessGrantsPage;
    }

    async function createAccessGrant(name: string, projectID: string): Promise<AccessGrant> {
        return await api.create(projectID, name, csrfToken.value);
    }

    async function deleteAccessGrants(ids: string[]): Promise<void> {
        await api.delete(ids, csrfToken.value);
    }

    async function getEdgeCredentials(accessGrant: string, isPublic = false): Promise<EdgeCredentials> {
        const url = projectsStore.state.selectedProject.edgeURLOverrides?.authService
            || configStore.state.config.gatewayCredentialsRequestURL;

        return await api.getGatewayCredentials(accessGrant, url, isPublic);
    }

    function setSearchQuery(query: string): void {
        state.cursor.search = query;
    }

    function setSortingBy(order: AccessGrantsOrderBy): void {
        state.cursor.order = order;
    }

    function setSortingDirection(direction: SortDirection): void {
        state.cursor.orderDirection = direction;
    }

    function clear(): void {
        state.allAGNames = [];
        state.cursor = new AccessGrantCursor();
        state.page = new AccessGrantsPage();
        state.accessGrantsWebWorker = null;
    }

    return {
        state,
        getAllAGNames,
        setWorker,
        getAccessGrants,
        createAccessGrant,
        deleteAccessGrants,
        getEdgeCredentials,
        setSearchQuery,
        setSortingBy,
        setSortingDirection,
        clear,
    };
});
