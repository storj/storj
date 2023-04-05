// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import S3, { Bucket } from 'aws-sdk/clients/s3';

import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { FilesState } from '@/store/modules/files';
import { StoreModule } from '@/types/store';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_GATEWAY_CREDENTIALS_FOR_DELETE: 'setGatewayCredentialsForDelete',
    SET_GATEWAY_CREDENTIALS_FOR_CREATE: 'setGatewayCredentialsForCreate',
    SET_API_KEY: 'setApiKey',
    SET_S3_CLIENT: 'setS3Client',
    SET_PASSPHRASE: 'setPassphrase',
    SET_FILE_COMPONENT_BUCKET_NAME: 'setFileComponentBucketName',
    FETCH_BUCKETS: 'fetchBuckets',
    CREATE_BUCKET: 'createBucket',
    CREATE_BUCKET_WITH_NO_PASSPHRASE: 'createBucketWithNoPassphrase',
    DELETE_BUCKET: 'deleteBucket',
    GET_OBJECTS_COUNT: 'getObjectsCount',
    CHECK_ONGOING_UPLOADS: 'checkOngoingUploads',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'SET_GATEWAY_CREDENTIALS',
    SET_GATEWAY_CREDENTIALS_FOR_DELETE: 'SET_GATEWAY_CREDENTIALS_FOR_DELETE',
    SET_GATEWAY_CREDENTIALS_FOR_CREATE: 'SET_GATEWAY_CREDENTIALS_FOR_CREATE',
    SET_API_KEY: 'SET_API_KEY',
    CLEAR: 'CLEAR_OBJECTS',
    SET_S3_CLIENT: 'SET_S3_CLIENT',
    SET_S3_CLIENT_FOR_DELETE: 'SET_S3_CLIENT_FOR_DELETE',
    SET_S3_CLIENT_FOR_CREATE: 'SET_S3_CLIENT_FOR_CREATE',
    SET_BUCKETS: 'SET_BUCKETS',
    SET_FILE_COMPONENT_BUCKET_NAME: 'SET_FILE_COMPONENT_BUCKET_NAME',
    SET_PASSPHRASE: 'SET_PASSPHRASE',
    SET_PROMPT_FOR_PASSPHRASE: 'SET_PROMPT_FOR_PASSPHRASE',
    SET_LEAVE_ROUTE: 'SET_LEAVE_ROUTE',
};

const {
    CLEAR,
    SET_API_KEY,
    SET_GATEWAY_CREDENTIALS,
    SET_GATEWAY_CREDENTIALS_FOR_DELETE,
    SET_GATEWAY_CREDENTIALS_FOR_CREATE,
    SET_S3_CLIENT,
    SET_S3_CLIENT_FOR_DELETE,
    SET_S3_CLIENT_FOR_CREATE,
    SET_BUCKETS,
    SET_PASSPHRASE,
    SET_PROMPT_FOR_PASSPHRASE,
    SET_FILE_COMPONENT_BUCKET_NAME,
    SET_LEAVE_ROUTE,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public apiKey = '';
    public gatewayCredentials: EdgeCredentials = new EdgeCredentials();
    public gatewayCredentialsForDelete: EdgeCredentials = new EdgeCredentials();
    public gatewayCredentialsForCreate: EdgeCredentials = new EdgeCredentials();
    public s3Client: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public s3ClientForDelete: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public s3ClientForCreate: S3 = new S3({
        s3ForcePathStyle: true,
        signatureVersion: 'v4',
        httpOptions: { timeout: 0 },
    });
    public bucketsList: Bucket[] = [];
    public passphrase = '';
    public promptForPassphrase = true;
    public fileComponentBucketName = '';
    public leaveRoute = '';
}

interface ObjectsContext {
    state: ObjectsState
    commit: (string, ...unknown) => void
    dispatch: (string, ...unknown) => Promise<any> // eslint-disable-line @typescript-eslint/no-explicit-any
    rootState: {
        files: FilesState
    }
    rootGetters: {
        worker: Worker,
        selectedProject: {
            id: string,
        }
    }
}

export const FILE_BROWSER_AG_NAME = 'Web file browser API key';

/**
 * Creates objects module with all dependencies.
 */
export function makeObjectsModule(): StoreModule<ObjectsState, ObjectsContext> {
    return {
        state: new ObjectsState(),
        mutations: {
            [SET_API_KEY](state: ObjectsState, apiKey: string) {
                state.apiKey = apiKey;
            },
            [SET_GATEWAY_CREDENTIALS](state: ObjectsState, credentials: EdgeCredentials) {
                state.gatewayCredentials = credentials;
            },
            [SET_GATEWAY_CREDENTIALS_FOR_DELETE](state: ObjectsState, credentials: EdgeCredentials) {
                state.gatewayCredentialsForDelete = credentials;
            },
            [SET_GATEWAY_CREDENTIALS_FOR_CREATE](state: ObjectsState, credentials: EdgeCredentials) {
                state.gatewayCredentialsForCreate = credentials;
            },
            [SET_S3_CLIENT](state: ObjectsState) {
                const s3Config = {
                    accessKeyId: state.gatewayCredentials.accessKeyId,
                    secretAccessKey: state.gatewayCredentials.secretKey,
                    endpoint: state.gatewayCredentials.endpoint,
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                };

                state.s3Client = new S3(s3Config);
            },
            [SET_S3_CLIENT_FOR_DELETE](state: ObjectsState) {
                const s3Config = {
                    accessKeyId: state.gatewayCredentialsForDelete.accessKeyId,
                    secretAccessKey: state.gatewayCredentialsForDelete.secretKey,
                    endpoint: state.gatewayCredentialsForDelete.endpoint,
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                };

                state.s3ClientForDelete = new S3(s3Config);
            },
            [SET_S3_CLIENT_FOR_CREATE](state: ObjectsState) {
                const s3Config = {
                    accessKeyId: state.gatewayCredentialsForCreate.accessKeyId,
                    secretAccessKey: state.gatewayCredentialsForCreate.secretKey,
                    endpoint: state.gatewayCredentialsForCreate.endpoint,
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                };

                state.s3ClientForCreate = new S3(s3Config);
            },
            [SET_BUCKETS](state: ObjectsState, buckets: Bucket[]) {
                state.bucketsList = buckets;
            },
            [SET_PASSPHRASE](state: ObjectsState, passphrase: string) {
                state.passphrase = passphrase;
            },
            [SET_PROMPT_FOR_PASSPHRASE](state: ObjectsState, value: boolean) {
                state.promptForPassphrase = value;
            },
            [SET_FILE_COMPONENT_BUCKET_NAME](state: ObjectsState, bucketName: string) {
                state.fileComponentBucketName = bucketName;
            },
            [SET_LEAVE_ROUTE](state: ObjectsState, leaveRoute: string) {
                state.leaveRoute = leaveRoute;
            },
            [CLEAR](state: ObjectsState) {
                state.apiKey = '';
                state.passphrase = '';
                state.promptForPassphrase = true;
                state.gatewayCredentials = new EdgeCredentials();
                state.gatewayCredentialsForDelete = new EdgeCredentials();
                state.gatewayCredentialsForCreate = new EdgeCredentials();
                state.s3Client = new S3({
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                });
                state.s3ClientForDelete = new S3({
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                });
                state.s3ClientForCreate = new S3({
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    httpOptions: { timeout: 0 },
                });
                state.bucketsList = [];
                state.fileComponentBucketName = '';
                state.leaveRoute = '';
            },
        },
        actions: {
            setApiKey: function({ commit }: ObjectsContext, apiKey: string): void {
                commit(SET_API_KEY, apiKey);
            },
            setGatewayCredentials: function({ commit }: ObjectsContext, credentials: EdgeCredentials): void {
                commit(SET_GATEWAY_CREDENTIALS, credentials);
            },
            setGatewayCredentialsForDelete: function({ commit }: ObjectsContext, credentials: EdgeCredentials): void {
                commit(SET_GATEWAY_CREDENTIALS_FOR_DELETE, credentials);
                commit(SET_S3_CLIENT_FOR_DELETE);
            },
            setGatewayCredentialsForCreate: function({ commit }: ObjectsContext, credentials: EdgeCredentials): void {
                commit(SET_GATEWAY_CREDENTIALS_FOR_CREATE, credentials);
                commit(SET_S3_CLIENT_FOR_CREATE);
            },
            setS3Client: async function({ commit, dispatch, state, rootGetters }: ObjectsContext): Promise<void> {
                const agStore = useAccessGrantsStore();

                if (!state.apiKey) {
                    await agStore.deleteAccessGrantByNameAndProjectID(FILE_BROWSER_AG_NAME, rootGetters.selectedProject.id);
                    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(FILE_BROWSER_AG_NAME, rootGetters.selectedProject.id);
                    commit(SET_API_KEY, cleanAPIKey.secret);
                }

                const now = new Date();
                const inThreeDays = new Date(now.setDate(now.getDate() + 3));

                const worker = agStore.state.accessGrantsWebWorker;
                if (!worker) {
                    throw new Error ('Worker is not set');
                }

                worker.onerror = (error: ErrorEvent) => {
                    throw new Error(error.message);
                };

                await worker.postMessage({
                    'type': 'SetPermission',
                    'isDownload': true,
                    'isUpload': true,
                    'isList': true,
                    'isDelete': true,
                    'notAfter': inThreeDays.toISOString(),
                    'buckets': [],
                    'apiKey': state.apiKey,
                });

                const grantEvent: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
                if (grantEvent.data.error) {
                    throw new Error(grantEvent.data.error);
                }

                const salt = await dispatch(PROJECTS_ACTIONS.GET_SALT, rootGetters.selectedProject.id, { root: true });
                const appStore = useAppStore();
                const satelliteNodeURL: string = appStore.state.config.satelliteNodeURL;

                if (!state.passphrase) {
                    throw new Error('Passphrase can\'t be empty');
                }

                worker.postMessage({
                    'type': 'GenerateAccess',
                    'apiKey': grantEvent.data.value,
                    'passphrase': state.passphrase,
                    'salt': salt,
                    'satelliteNodeURL': satelliteNodeURL,
                });

                const accessGrantEvent: MessageEvent = await new Promise(resolve => worker.onmessage = resolve);
                if (accessGrantEvent.data.error) {
                    throw new Error(accessGrantEvent.data.error);
                }

                const accessGrant = accessGrantEvent.data.value;

                const gatewayCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
                commit(SET_GATEWAY_CREDENTIALS, gatewayCredentials);
                commit(SET_S3_CLIENT);
            },
            setPassphrase: function({ commit }: ObjectsContext, passphrase: string): void {
                commit(SET_PASSPHRASE, passphrase);
            },
            setFileComponentBucketName: function({ commit }: ObjectsContext, bucketName: string): void {
                commit(SET_FILE_COMPONENT_BUCKET_NAME, bucketName);
            },
            fetchBuckets: async function(ctx): Promise<void> {
                const result = await ctx.state.s3Client.listBuckets().promise();

                ctx.commit(SET_BUCKETS, result.Buckets);
            },
            createBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.createBucket({
                    Bucket: name,
                }).promise();
            },
            createBucketWithNoPassphrase: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3ClientForCreate.createBucket({
                    Bucket: name,
                }).promise();
            },
            deleteBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3ClientForDelete.deleteBucket({
                    Bucket: name,
                }).promise();
            },
            getObjectsCount: async function(ctx, name: string): Promise<number> {
                const response =  await ctx.state.s3Client.listObjectsV2({
                    Bucket: name,
                }).promise();

                return response.KeyCount === undefined ? 0 : response.KeyCount;
            },
            clearObjects: function({ commit }: ObjectsContext): void {
                commit(CLEAR);
            },
            checkOngoingUploads: function({ commit, dispatch, rootState }: ObjectsContext, leaveRoute: string): boolean {
                if (!rootState.files.uploading.length) {
                    return false;
                }

                commit(SET_LEAVE_ROUTE, leaveRoute);
                const appStore = useAppStore();
                appStore.updateActiveModal(MODALS.uploadCancelPopup);

                return true;
            },
        },
    };
}
