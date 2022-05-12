// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    S3Client,
    Bucket,
    ListBucketsCommand,
    CreateBucketCommand,
    DeleteBucketCommand
} from '@aws-sdk/client-s3';

import { EdgeCredentials } from '@/types/accessGrants';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { FilesState } from '@/store/modules/files';
import {StoreModule} from "@/types/store";
import {SignatureV4} from "@aws-sdk/signature-v4";

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_API_KEY: 'setApiKey',
    SET_S3_CLIENT: 'setS3Client',
    SET_PASSPHRASE: 'setPassphrase',
    SET_FILE_COMPONENT_BUCKET_NAME: 'setFileComponentBucketName',
    FETCH_BUCKETS: 'fetchBuckets',
    CREATE_BUCKET: 'createBucket',
    CREATE_DEMO_BUCKET: 'createDemoBucket',
    DELETE_BUCKET: 'deleteBucket',
    CHECK_ONGOING_UPLOADS: 'checkOngoingUploads',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_API_KEY: 'setApiKey',
    CLEAR: 'clearObjects',
    SET_S3_CLIENT: 'setS3Client',
    SET_BUCKETS: 'setBuckets',
    SET_FILE_COMPONENT_BUCKET_NAME: 'setFileComponentBucketName',
    SET_PASSPHRASE: 'setPassphrase',
    SET_LEAVE_ROUTE: 'setLeaveRoute',
};

export const DEMO_BUCKET_NAME = 'demo-bucket';

const {
    CLEAR,
    SET_API_KEY,
    SET_GATEWAY_CREDENTIALS,
    SET_S3_CLIENT,
    SET_BUCKETS,
    SET_PASSPHRASE,
    SET_FILE_COMPONENT_BUCKET_NAME,
    SET_LEAVE_ROUTE,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public apiKey = '';
    public gatewayCredentials: EdgeCredentials = new EdgeCredentials();
    public s3Client: S3Client = new S3Client({
        forcePathStyle: true,
        signerConstructor: SignatureV4,
        // TODO: httpOptions: { timeout: 0 },
    });
    public bucketsList: Bucket[] = [];
    public passphrase = '';
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
            [SET_S3_CLIENT](state: ObjectsState) {
                state.s3Client = new S3Client({
                    credentials: {
                        accessKeyId: state.gatewayCredentials.accessKeyId,
                        secretAccessKey: state.gatewayCredentials.secretKey,
                    },
                    endpoint: state.gatewayCredentials.endpoint,
                    forcePathStyle: true,
                    signerConstructor: SignatureV4,
                    // TODO: httpOptions: { timeout: 0 },
                });
            },
            [SET_BUCKETS](state: ObjectsState, buckets: Bucket[]) {
                state.bucketsList = buckets;
            },
            [SET_PASSPHRASE](state: ObjectsState, passphrase: string) {
                state.passphrase = passphrase;
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
                state.gatewayCredentials = new EdgeCredentials();
                state.s3Client = new S3Client({
                    forcePathStyle: true,
                    signerConstructor: SignatureV4,
                    // TODO: httpOptions: { timeout: 0 },
                });
                state.bucketsList = [];
                state.fileComponentBucketName = '';
            },
        },
        actions: {
            setApiKey: function({commit}: ObjectsContext, apiKey: string): void {
                commit(SET_API_KEY, apiKey);
            },
            setGatewayCredentials: function({commit}: ObjectsContext, credentials: EdgeCredentials): void {
                commit(SET_GATEWAY_CREDENTIALS, credentials);
            },
            setS3Client: function({commit}: ObjectsContext): void {
                commit(SET_S3_CLIENT);
            },
            setPassphrase: function({commit}: ObjectsContext, passphrase: string): void {
                commit(SET_PASSPHRASE, passphrase);
            },
            setFileComponentBucketName: function({commit}: ObjectsContext, bucketName: string): void {
                commit(SET_FILE_COMPONENT_BUCKET_NAME, bucketName);
            },
            fetchBuckets: async function(ctx): Promise<void> {
                const result = await ctx.state.s3Client.send(new ListBucketsCommand({}));
                ctx.commit(SET_BUCKETS, result.Buckets);
            },
            createBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.send(new CreateBucketCommand({
                    Bucket: name,
                }))
            },
            createDemoBucket: async function(ctx): Promise<void> {
                await ctx.state.s3Client.send(new CreateBucketCommand({
                    Bucket: DEMO_BUCKET_NAME,
                }))
            },
            deleteBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.send(
                    new DeleteBucketCommand({
                        Bucket: name,
                    }));
            },
            clearObjects: function({commit}: ObjectsContext): void {
                commit(CLEAR);
            },
            checkOngoingUploads: function({commit, dispatch, rootState}: ObjectsContext, leaveRoute: string): boolean {
                if (!rootState.files.uploading.length) {
                    return false;
                }

                commit(SET_LEAVE_ROUTE, leaveRoute);
                dispatch(APP_STATE_ACTIONS.TOGGLE_UPLOAD_CANCEL_POPUP, null, {root: true});

                return true;
            },
        },
    };
}
