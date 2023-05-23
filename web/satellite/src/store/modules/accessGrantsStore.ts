// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import {
    AccessGrant,
    AccessGrantCursor,
    AccessGrantsOrderBy,
    AccessGrantsPage,
    DurationPermission,
    EdgeCredentials,
} from '@/types/accessGrants';
import { SortDirection } from '@/types/common';
import { AccessGrantsApiGql } from '@/api/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

class AccessGrantsState {
    public cursor: AccessGrantCursor = new AccessGrantCursor();
    public page: AccessGrantsPage = new AccessGrantsPage();
    public selectedAccessGrantsIds: string[] = [];
    public selectedBucketNames: string[] = [];
    public permissionNotBefore: Date | null = null;
    public permissionNotAfter: Date | null = null;
    public isDownload = true;
    public isUpload = true;
    public isList = true;
    public isDelete = true;
    public edgeCredentials: EdgeCredentials = new EdgeCredentials();
    public accessGrantsWebWorker: Worker | null = null;
    public isAccessGrantsWebWorkerReady = false;
    public accessNameToDelete = '';
}

export const useAccessGrantsStore = defineStore('accessGrants', () => {
    const api = new AccessGrantsApiGql();

    const state = reactive<AccessGrantsState>(new AccessGrantsState());

    const configStore = useConfigStore();

    async function startWorker(): Promise<void> {
        // TODO(vitalii): create an issue here https://github.com/vitejs/vite
        // about worker chunk being auto removed after rebuild in watch mode if using new URL constructor.
        // const worker = new Worker(new URL('@/utils/accessGrant.worker.js', import.meta.url));
        const worker = new Worker('/static/src/utils/accessGrant.worker.js');
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

    async function getAccessGrants(pageNumber: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<AccessGrantsPage> {
        state.cursor.page = pageNumber;
        state.cursor.limit = limit;

        const accessGrantsPage: AccessGrantsPage = await api.get(projectID, state.cursor);

        state.page = accessGrantsPage;
        state.page.accessGrants = state.page.accessGrants.map(accessGrant => {
            if (state.selectedAccessGrantsIds.includes(accessGrant.id)) {
                accessGrant.isSelected = true;
            }

            return accessGrant;
        });

        return accessGrantsPage;
    }

    async function createAccessGrant(name: string, projectID: string): Promise<AccessGrant> {
        return await api.create(projectID, name);
    }

    async function deleteAccessGrants(): Promise<void> {
        await api.delete(state.selectedAccessGrantsIds);
    }

    async function deleteAccessGrantByNameAndProjectID(name: string, projectID: string): Promise<void> {
        await api.deleteByNameAndProjectID(name, projectID);
    }

    async function getEdgeCredentials(accessGrant: string, optionalURL?: string, isPublic?: boolean): Promise<EdgeCredentials> {
        const url = optionalURL || configStore.state.config.gatewayCredentialsRequestURL;
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

    function setAccessNameToDelete(name: string): void {
        state.accessNameToDelete = name;
    }

    function setDurationPermission(permission: DurationPermission): void {
        state.permissionNotBefore = permission.notBefore;
        state.permissionNotAfter = permission.notAfter;
    }

    function toggleSelection(accessGrant: AccessGrant): void {
        if (!state.selectedAccessGrantsIds.includes(accessGrant.id)) {
            state.page.accessGrants.forEach((grant: AccessGrant) => {
                if (grant.id === accessGrant.id) {
                    grant.isSelected = true;
                }
            });
            state.selectedAccessGrantsIds.push(accessGrant.id);

            return;
        }

        state.page.accessGrants.forEach((grant: AccessGrant) => {
            if (grant.id === accessGrant.id) {
                grant.isSelected = false;
            }
        });
        state.selectedAccessGrantsIds = state.selectedAccessGrantsIds.filter(accessGrantId => {
            return accessGrant.id !== accessGrantId;
        });
    }

    function toggleBucketSelection(bucketName: string): void {
        if (!state.selectedBucketNames.includes(bucketName)) {
            state.selectedBucketNames.push(bucketName);

            return;
        }

        state.selectedBucketNames = state.selectedBucketNames.filter(name => {
            return bucketName !== name;
        });
    }

    function setSortingDirection(direction: SortDirection): void {
        state.cursor.orderDirection = direction;
    }

    function toggleSortingDirection(): void {
        let direction = SortDirection.DESCENDING;
        if (state.cursor.orderDirection === SortDirection.DESCENDING) {
            direction = SortDirection.ASCENDING;
        }
        state.cursor.orderDirection = direction;
    }

    function toggleIsDownloadPermission(): void {
        state.isDownload = !state.isDownload;
    }

    function toggleIsUploadPermission(): void {
        state.isUpload = !state.isUpload;
    }

    function toggleIsListPermission(): void {
        state.isList = !state.isList;
    }

    function toggleIsDeletePermission(): void {
        state.isDelete = !state.isDelete;
    }

    function clearSelection(): void {
        state.selectedBucketNames = [];
        state.selectedAccessGrantsIds = [];
        state.page.accessGrants = state.page.accessGrants.map((accessGrant: AccessGrant) => {
            accessGrant.isSelected = false;

            return accessGrant;
        });
    }

    function clear(): void {
        state.cursor = new AccessGrantCursor();
        state.page = new AccessGrantsPage();
        state.selectedAccessGrantsIds = [];
        state.selectedBucketNames = [];
        state.permissionNotBefore = null;
        state.permissionNotAfter = null;
        state.edgeCredentials = new EdgeCredentials();
        state.isDownload = true;
        state.isUpload = true;
        state.isList = true;
        state.isDelete = true;
        state.accessGrantsWebWorker = null;
        state.isAccessGrantsWebWorkerReady = false;
    }

    const selectedAccessGrants = computed((): AccessGrant[] => {
        return state.page.accessGrants.filter((grant: AccessGrant) => grant.isSelected);
    });

    return {
        state,
        selectedAccessGrants,
        startWorker,
        stopWorker,
        getAccessGrants,
        createAccessGrant,
        deleteAccessGrants,
        deleteAccessGrantByNameAndProjectID,
        getEdgeCredentials,
        setSearchQuery,
        setSortingBy,
        setAccessNameToDelete,
        setSortingDirection,
        toggleSortingDirection,
        setDurationPermission,
        toggleSelection,
        toggleBucketSelection,
        toggleIsDownloadPermission,
        toggleIsUploadPermission,
        toggleIsListPermission,
        toggleIsDeletePermission,
        clearSelection,
        clear,
    };
});
