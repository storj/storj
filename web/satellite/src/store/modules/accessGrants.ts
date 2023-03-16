// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    AccessGrant,
    AccessGrantCursor,
    AccessGrantsApi,
    AccessGrantsOrderBy,
    AccessGrantsPage,
    AccessGrantsWorkerFactory,
    DurationPermission,
    EdgeCredentials,
} from '@/types/accessGrants';
import { SortDirection } from '@/types/common';
import { StoreModule } from '@/types/store';

export const ACCESS_GRANTS_ACTIONS = {
    FETCH: 'fetchAccessGrants',
    CREATE: 'createAccessGrant',
    DELETE: 'deleteAccessGrants',
    DELETE_BY_NAME_AND_PROJECT_ID: 'deleteAccessGrantsByNameAndProjectID',
    CLEAR: 'clearAccessGrants',
    GET_GATEWAY_CREDENTIALS: 'getGatewayCredentials',
    SET_ACCESS_GRANTS_WEB_WORKER: 'setAccessGrantsWebWorker',
    STOP_ACCESS_GRANTS_WEB_WORKER: 'stopAccessGrantsWebWorker',
    SET_SEARCH_QUERY: 'setAccessGrantsSearchQuery',
    SET_SORT_BY: 'setAccessGrantsSortingBy',
    SET_SORT_DIRECTION: 'setAccessGrantsSortingDirection',
    TOGGLE_SORT_DIRECTION: 'toggleAccessGrantsSortingDirection',
    SET_DURATION_PERMISSION: 'setAccessGrantsDurationPermission',
    TOGGLE_SELECTION: 'toggleAccessGrantsSelection',
    TOGGLE_BUCKET_SELECTION: 'toggleBucketSelection',
    CLEAR_SELECTION: 'clearAccessGrantsSelection',
};

export const ACCESS_GRANTS_MUTATIONS = {
    SET_PAGE: 'setAccessGrants',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANTS_WEB_WORKER: 'setAccessGrantsWebWorker',
    STOP_ACCESS_GRANTS_WEB_WORKER: 'stopAccessGrantsWebWorker',
    TOGGLE_SELECTION: 'toggleAccessGrantsSelection',
    TOGGLE_BUCKET_SELECTION: 'toggleBucketSelection',
    CLEAR_SELECTION: 'clearAccessGrantsSelection',
    CLEAR: 'clearAccessGrants',
    CHANGE_SORT_ORDER: 'changeAccessGrantsSortOrder',
    CHANGE_SORT_ORDER_DIRECTION: 'changeAccessGrantsSortOrderDirection',
    SET_SEARCH_QUERY: 'setAccessGrantsSearchQuery',
    SET_PAGE_NUMBER: 'setAccessGrantsPage',
    SET_DURATION_PERMISSION: 'setAccessGrantsDurationPermission',
    TOGGLE_IS_DOWNLOAD_PERMISSION: 'toggleAccessGrantsIsDownloadPermission',
    TOGGLE_IS_UPLOAD_PERMISSION: 'toggleAccessGrantsIsUploadPermission',
    TOGGLE_IS_LIST_PERMISSION: 'toggleAccessGrantsIsListPermission',
    TOGGLE_IS_DELETE_PERMISSION: 'toggleAccessGrantsIsDeletePermission',
};

const {
    SET_PAGE,
    TOGGLE_SELECTION,
    TOGGLE_BUCKET_SELECTION,
    CLEAR_SELECTION,
    CLEAR,
    CHANGE_SORT_ORDER,
    CHANGE_SORT_ORDER_DIRECTION,
    SET_SEARCH_QUERY,
    SET_PAGE_NUMBER,
    SET_DURATION_PERMISSION,
    TOGGLE_IS_DOWNLOAD_PERMISSION,
    TOGGLE_IS_UPLOAD_PERMISSION,
    TOGGLE_IS_LIST_PERMISSION,
    TOGGLE_IS_DELETE_PERMISSION,
    SET_GATEWAY_CREDENTIALS,
    SET_ACCESS_GRANTS_WEB_WORKER,
    STOP_ACCESS_GRANTS_WEB_WORKER,
} = ACCESS_GRANTS_MUTATIONS;

export class AccessGrantsState {
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
    public gatewayCredentials: EdgeCredentials = new EdgeCredentials();
    public accessGrantsWebWorker: Worker | null = null;
    public isAccessGrantsWebWorkerReady = false;
}

interface AccessGrantsContext {
    state: AccessGrantsState
    commit: (string, ...unknown) => void
    rootGetters: {
        selectedProject: {
            id: string
        }
    }
}

/**
 * creates access grants module with all dependencies
 *
 * @param api - accessGrants api
 */
export function makeAccessGrantsModule(api: AccessGrantsApi, workerFactory?: AccessGrantsWorkerFactory): StoreModule<AccessGrantsState, AccessGrantsContext> {
    return {
        state: new AccessGrantsState(),
        mutations: {
            [SET_ACCESS_GRANTS_WEB_WORKER](state: AccessGrantsState, worker: Worker): void {
                state.accessGrantsWebWorker = worker;
                state.isAccessGrantsWebWorkerReady = true;
            },
            [STOP_ACCESS_GRANTS_WEB_WORKER](state: AccessGrantsState): void {
                state.accessGrantsWebWorker?.terminate();
                state.accessGrantsWebWorker = null;
                state.isAccessGrantsWebWorkerReady = false;
            },
            [SET_PAGE](state: AccessGrantsState, page: AccessGrantsPage) {
                state.page = page;
                state.page.accessGrants = state.page.accessGrants.map(accessGrant => {
                    if (state.selectedAccessGrantsIds.includes(accessGrant.id)) {
                        accessGrant.isSelected = true;
                    }

                    return accessGrant;
                });
            },
            [SET_GATEWAY_CREDENTIALS](state: AccessGrantsState, credentials: EdgeCredentials) {
                state.gatewayCredentials = credentials;
            },
            [SET_PAGE_NUMBER](state: AccessGrantsState, pageNumber: number) {
                state.cursor.page = pageNumber;
            },
            [SET_SEARCH_QUERY](state: AccessGrantsState, search: string) {
                state.cursor.search = search;
            },
            [SET_DURATION_PERMISSION](state: AccessGrantsState, permission: DurationPermission) {
                state.permissionNotBefore = permission.notBefore;
                state.permissionNotAfter = permission.notAfter;
            },
            [TOGGLE_IS_DOWNLOAD_PERMISSION](state: AccessGrantsState) {
                state.isDownload = !state.isDownload;
            },
            [TOGGLE_IS_UPLOAD_PERMISSION](state: AccessGrantsState) {
                state.isUpload = !state.isUpload;
            },
            [TOGGLE_IS_LIST_PERMISSION](state: AccessGrantsState) {
                state.isList = !state.isList;
            },
            [TOGGLE_IS_DELETE_PERMISSION](state: AccessGrantsState) {
                state.isDelete = !state.isDelete;
            },
            [CHANGE_SORT_ORDER](state: AccessGrantsState, order: AccessGrantsOrderBy) {
                state.cursor.order = order;
            },
            [CHANGE_SORT_ORDER_DIRECTION](state: AccessGrantsState, direction: SortDirection) {
                state.cursor.orderDirection = direction;
            },
            [TOGGLE_SELECTION](state: AccessGrantsState, accessGrant: AccessGrant) {
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
            },
            [TOGGLE_BUCKET_SELECTION](state: AccessGrantsState, bucketName: string) {
                if (!state.selectedBucketNames.includes(bucketName)) {
                    state.selectedBucketNames.push(bucketName);

                    return;
                }

                state.selectedBucketNames = state.selectedBucketNames.filter(name => {
                    return bucketName !== name;
                });
            },
            [CLEAR_SELECTION](state: AccessGrantsState) {
                state.selectedBucketNames = [];
                state.selectedAccessGrantsIds = [];
                state.page.accessGrants = state.page.accessGrants.map((accessGrant: AccessGrant) => {
                    accessGrant.isSelected = false;

                    return accessGrant;
                });
            },
            [CLEAR](state: AccessGrantsState) {
                state.cursor = new AccessGrantCursor();
                state.page = new AccessGrantsPage();
                state.selectedAccessGrantsIds = [];
                state.selectedBucketNames = [];
                state.permissionNotBefore = null;
                state.permissionNotAfter = null;
                state.gatewayCredentials = new EdgeCredentials();
                state.isDownload = true;
                state.isUpload = true;
                state.isList = true;
                state.isDelete = true;
                state.accessGrantsWebWorker = null;
                state.isAccessGrantsWebWorkerReady = false;
            },
        },
        actions: {
            setAccessGrantsWebWorker: async function ({ commit }: AccessGrantsContext): Promise<void> {
                if (!workerFactory) {
                    throw new Error('Worker not supported');
                }

                const worker = workerFactory.create();
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

                commit(SET_ACCESS_GRANTS_WEB_WORKER, worker);
            },
            stopAccessGrantsWebWorker: function ({ commit }: AccessGrantsContext): void {
                commit(STOP_ACCESS_GRANTS_WEB_WORKER);
            },
            fetchAccessGrants: async function ({ commit, rootGetters, state }: AccessGrantsContext, pageNumber: number): Promise<AccessGrantsPage> {
                const projectId = rootGetters.selectedProject.id;
                commit(SET_PAGE_NUMBER, pageNumber);

                const accessGrantsPage: AccessGrantsPage = await api.get(projectId, state.cursor);
                commit(SET_PAGE, accessGrantsPage);

                return accessGrantsPage;
            },
            createAccessGrant: async function ({ rootGetters }: AccessGrantsContext, name: string): Promise<AccessGrant> {
                return await api.create(rootGetters.selectedProject.id, name);
            },
            deleteAccessGrants: async function ({ state }: AccessGrantsContext): Promise<void> {
                await api.delete(state.selectedAccessGrantsIds);
            },
            deleteAccessGrantsByNameAndProjectID: async function ({ rootGetters }: AccessGrantsContext, name: string): Promise<void> {
                await api.deleteByNameAndProjectID(name, rootGetters.selectedProject.id);
            },
            getGatewayCredentials: async function ({ commit }: AccessGrantsContext, payload): Promise<EdgeCredentials> {
                const credentials: EdgeCredentials = await api.getGatewayCredentials(payload.accessGrant, payload.optionalURL, payload.isPublic);

                commit(SET_GATEWAY_CREDENTIALS, credentials);

                return credentials;
            },
            setAccessGrantsSearchQuery: function ({ commit }: AccessGrantsContext, search: string) {
                commit(SET_SEARCH_QUERY, search);
            },
            setAccessGrantsSortingBy: function ({ commit }: AccessGrantsContext, order: AccessGrantsOrderBy) {
                commit(CHANGE_SORT_ORDER, order);
            },
            setAccessGrantsSortingDirection: function ({ commit }: AccessGrantsContext, direction: SortDirection) {
                commit(CHANGE_SORT_ORDER_DIRECTION, direction);
            },
            toggleAccessGrantsSortingDirection: function ({ commit, state }: AccessGrantsContext) {
                let direction = SortDirection.DESCENDING;
                if (state.cursor.orderDirection === SortDirection.DESCENDING) {
                    direction = SortDirection.ASCENDING;
                }
                commit(CHANGE_SORT_ORDER_DIRECTION, direction);
            },
            setAccessGrantsDurationPermission: function ({ commit }: AccessGrantsContext, permission: DurationPermission) {
                commit(SET_DURATION_PERMISSION, permission);
            },
            toggleAccessGrantsSelection: function ({ commit }: AccessGrantsContext, accessGrant: AccessGrant): void {
                commit(TOGGLE_SELECTION, accessGrant);
            },
            toggleBucketSelection: function ({ commit }: AccessGrantsContext, bucketName: string): void {
                commit(TOGGLE_BUCKET_SELECTION, bucketName);
            },
            clearAccessGrantsSelection: function ({ commit }: AccessGrantsContext): void {
                commit(CLEAR_SELECTION);
            },
            clearAccessGrants: function ({ commit }: AccessGrantsContext): void {
                commit(CLEAR);
                commit(CLEAR_SELECTION);
            },
        },
        getters: {
            selectedAccessGrants: (state: AccessGrantsState) => state.page.accessGrants.filter((grant: AccessGrant) => grant.isSelected),
            worker: (state: AccessGrantsState) => state.accessGrantsWebWorker,
        },
    };
}
