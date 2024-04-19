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
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public accessGrantsWebWorker: Worker | null = null;
    public isAccessGrantsWebWorkerReady = false;
}

export const useAccessGrantsStore = defineStore('accessGrants', () => {
    const api = new AccessGrantsHttpApi();

    const state = reactive<AccessGrantsState>(new AccessGrantsState());

    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();

    async function startWorker(): Promise<void> {
        // TODO(vitalii): create an issue here https://github.com/vitejs/vite
        // about worker chunk being auto removed after rebuild in watch mode if using new URL constructor.
        let worker: Worker;
        if (import.meta.env.MODE === 'development') {
            worker = new Worker('/static/src/utils/accessGrant.worker.js');
        } else {
            worker = new Worker(new URL('@/utils/accessGrant.worker.js', import.meta.url));
        }

        worker.postMessage({ 'type': 'Setup' });

        const event: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
        if (event.data.error) {
            throw new Error(event.data.error);
        }

        if (event.data !== 'configured') {
            throw new Error('Failed to configure access grants web worker');
        }

        worker.onerror = (error: ErrorEvent) => {
            throw new Error(`Failed to configure access grants web worker. ${error.message}`);
        };

        state.accessGrantsWebWorker = worker;
        state.isAccessGrantsWebWorkerReady = true;
    }

    function stopWorker(): void {
        state.accessGrantsWebWorker?.terminate();
        state.accessGrantsWebWorker = null;
        state.isAccessGrantsWebWorkerReady = false;
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
        return await api.create(projectID, name);
    }

    async function deleteAccessGrants(ids: string[]): Promise<void> {
        await api.delete(ids);
    }

    async function getEdgeCredentials(accessGrant: string, isPublic?: boolean): Promise<EdgeCredentials> {
        const url = projectsStore.state.selectedProject.edgeURLOverrides?.authService
            || configStore.state.config.gatewayCredentialsRequestURL;
        const credentials: EdgeCredentials = await api.getGatewayCredentials(accessGrant, url, isPublic);

        state.edgeCredentials = credentials;

        return credentials;
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
        state.edgeCredentials = new EdgeCredentials();
        state.accessGrantsWebWorker = null;
        state.isAccessGrantsWebWorkerReady = false;
    }

    const selectedAccessGrants = computed((): AccessGrant[] => {
        return state.page.accessGrants.filter((grant: AccessGrant) => grant.isSelected);
    });

    return {
        state,
        selectedAccessGrants,
        getAllAGNames,
        startWorker,
        stopWorker,
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
