// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { GatewayCredentials } from '@/types/accessGrants';
import {
    Bucket,
    CreateBucketCommand,
    DeleteBucketCommand,
    ListBucketsCommand,
    ListBucketsOutput,
    S3Client,
} from '@aws-sdk/client-s3';

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
    SET_S3_CLIENT: 'setS3Client',
    SET_PASSPHRASE: 'setPassphrase',
    FETCH_BUCKETS: 'fetchBuckets',
    CREATE_BUCKET: 'createBucket',
    DELETE_BUCKET: 'deleteBucket',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
    CLEAR: 'clearObjects',
    SET_S3_CLIENT: 'setS3Client',
    SET_BUCKETS: 'setBuckets',
    SET_PASSPHRASE: 'setPassphrase',
};

const {
    CLEAR,
    SET_ACCESS_GRANT,
    SET_GATEWAY_CREDENTIALS,
    SET_S3_CLIENT,
    SET_BUCKETS,
    SET_PASSPHRASE,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public accessGrant: string = '';
    public gatewayCredentials: GatewayCredentials = new GatewayCredentials();
    public s3Client: S3Client = new S3Client({});
    public buckets: Bucket[] = [];
    public passphrase: string = '';
}

/**
 * Creates objects module with all dependencies.
 */
export function makeObjectsModule(): StoreModule<ObjectsState> {
    return {
        state: new ObjectsState(),
        mutations: {
            [SET_ACCESS_GRANT](state: ObjectsState, accessGrant: string) {
                state.accessGrant = accessGrant;
            },
            [SET_GATEWAY_CREDENTIALS](state: ObjectsState, credentials: GatewayCredentials) {
                state.gatewayCredentials = credentials;
            },
            [SET_S3_CLIENT](state: ObjectsState) {
                // TODO: use this for local testing. Remove after final implementation.
                // state.gatewayCredentials.accessKeyId = 'jwitszrc76z4amjcrinv4zjpnlia';
                // state.gatewayCredentials.secretKey = 'jyjufay7ddmwj6tlboyuj23yy4lqigfqa2ie25y526qmjj65khxzw';
                // state.gatewayCredentials.endpoint = 'https://gateway.tardigradeshare.io';

                const s3Config = {
                    credentials: {
                        accessKeyId: state.gatewayCredentials.accessKeyId,
                        secretAccessKey: state.gatewayCredentials.secretKey,
                    },
                    endpoint: state.gatewayCredentials.endpoint,
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                    region: 'REGION',
                };

                state.s3Client = new S3Client(s3Config);
            },
            [SET_BUCKETS](state: ObjectsState, buckets: Bucket[]) {
                state.buckets = buckets;
            },
            [SET_PASSPHRASE](state: ObjectsState, passphrase: string) {
                state.passphrase = passphrase;
            },
            [CLEAR](state: ObjectsState) {
                state.accessGrant = '';
                state.gatewayCredentials = new GatewayCredentials();
            },
        },
        actions: {
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
            fetchBuckets: async function(ctx): Promise<void> {
                const bucketsOutput: ListBucketsOutput = await ctx.state.s3Client.send(new ListBucketsCommand({
                    credentials: {
                        accessKeyId: ctx.state.gatewayCredentials.accessKeyId,
                        secretAccessKey: ctx.state.gatewayCredentials.secretKey,
                    },
                }));

                ctx.commit(SET_BUCKETS, bucketsOutput.Buckets);
            },
            createBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.send(new CreateBucketCommand({
                    Bucket: name,
                }));
            },
            deleteBucket: async function(ctx, name: string): Promise<void> {
                await ctx.state.s3Client.send(new DeleteBucketCommand({
                    Bucket: name,
                }));
            },
            clearObjects: function ({commit}: any): void {
                commit(CLEAR);
            },
        },
    };
}
