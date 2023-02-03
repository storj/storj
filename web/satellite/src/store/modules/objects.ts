// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import S3, { Bucket } from 'aws-sdk/clients/s3';

import { EdgeCredentials } from '@/types/accessGrants';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { FilesState } from '@/store/modules/files';
import { StoreModule } from '@/types/store';

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
}

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
            setS3Client: function({ commit }: ObjectsContext): void {
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
            clearObjects: function({ commit }: ObjectsContext): void {
                commit(CLEAR);
            },
            checkOngoingUploads: function({ commit, dispatch, rootState }: ObjectsContext, leaveRoute: string): boolean {
                if (!rootState.files.uploading.length) {
                    return false;
                }

                commit(SET_LEAVE_ROUTE, leaveRoute);
                dispatch(APP_STATE_ACTIONS.TOGGLE_UPLOAD_CANCEL_POPUP, null, { root: true });

                return true;
            },
        },
    };
}
