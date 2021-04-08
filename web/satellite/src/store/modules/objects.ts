// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import S3, { Bucket } from 'aws-sdk/clients/s3';

import { StoreModule } from '@/store';
import { GatewayCredentials } from '@/types/accessGrants';

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_API_KEY: 'setApiKey',
    SET_ACCESS_GRANT: 'setAccessGrant',
    SET_S3_CLIENT: 'setS3Client',
    SET_PASSPHRASE: 'setPassphrase',
    SET_FILE_COMPONENT_BUCKET_NAME: 'setFileComponentBucketName',
    FETCH_BUCKETS: 'fetchBuckets',
    CREATE_BUCKET: 'createBucket',
    DELETE_BUCKET: 'deleteBucket',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_API_KEY: 'setApiKey',
    SET_ACCESS_GRANT: 'setAccessGrant',
    CLEAR: 'clearObjects',
    SET_S3_CLIENT: 'setS3Client',
    SET_BUCKETS: 'setBuckets',
    SET_FILE_COMPONENT_BUCKET_NAME: 'setFileComponentBucketName',
    SET_PASSPHRASE: 'setPassphrase',
};

const {
    CLEAR,
    SET_API_KEY,
    SET_ACCESS_GRANT,
    SET_GATEWAY_CREDENTIALS,
    SET_S3_CLIENT,
    SET_BUCKETS,
    SET_PASSPHRASE,
    SET_FILE_COMPONENT_BUCKET_NAME,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public apiKey: string = '';
    public accessGrant: string = '';
    public gatewayCredentials: GatewayCredentials = new GatewayCredentials();
    public s3Client: S3 = new S3({});
    public bucketsList: Bucket[] = [];
    public passphrase: string = '';
    public fileComponentBucketName: string = '';
}

/**
 * Creates objects module with all dependencies.
 */
export function makeObjectsModule(): StoreModule<ObjectsState> {
    return {
        state: new ObjectsState(),
        mutations: {
            [SET_API_KEY](state: ObjectsState, apiKey: string) {
                state.apiKey = apiKey;
            },
            [SET_ACCESS_GRANT](state: ObjectsState, accessGrant: string) {
                state.accessGrant = accessGrant;
            },
            [SET_GATEWAY_CREDENTIALS](state: ObjectsState, credentials: GatewayCredentials) {
                state.gatewayCredentials = credentials;
            },
            [SET_S3_CLIENT](state: ObjectsState) {
                const s3Config = {
                    accessKeyId: state.gatewayCredentials.accessKeyId,
                    secretAccessKey: state.gatewayCredentials.secretKey,
                    endpoint: state.gatewayCredentials.endpoint,
                };

                state.s3Client = new S3(s3Config);
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
            [CLEAR](state: ObjectsState) {
                state.apiKey = '';
                state.passphrase = '';
                state.accessGrant = '';
                state.gatewayCredentials = new GatewayCredentials();
                state.s3Client = new S3({});
                state.bucketsList = [];
                state.fileComponentBucketName = '';
            },
        },
        actions: {
            setApiKey: function({commit}: any, apiKey: string): void {
                commit(SET_API_KEY, apiKey);
            },
            setAccessGrant: function({commit}: any, accessGrant: string): void {
                commit(SET_ACCESS_GRANT, accessGrant);
            },
            setGatewayCredentials: function({commit}: any, credentials: GatewayCredentials): void {
                commit(SET_GATEWAY_CREDENTIALS, credentials);
            },
            setS3Client: function({commit}: any): void {
                commit(SET_S3_CLIENT);
            },
            setPassphrase: function({commit}: any, passphrase: string): void {
                commit(SET_PASSPHRASE, passphrase);
            },
            setFileComponentBucketName: function({commit}: any, bucketName: string): void {
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
            deleteBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.deleteBucket({
                    Bucket: name,
                }).promise();
            },
            clearObjects: function ({commit}: any): void {
                commit(CLEAR);
            },
        },
    };
}
