// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { GatewayCredentials } from '@/types/accessGrants';
import * as AWS from '@aws-sdk/client-s3';

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
    SET_S3_CLIENT: 'setS3Client',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
    CLEAR: 'clearObjects',
    SET_S3_CLIENT: 'setS3Client',
};

const {
    CLEAR,
    SET_ACCESS_GRANT,
    SET_GATEWAY_CREDENTIALS,
    SET_S3_CLIENT,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public accessGrant: string = '';
    public gatewayCredentials: GatewayCredentials = new GatewayCredentials();
    public s3Client: AWS.S3 = new AWS.S3({});
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
                const s3Config = {
                    accessKeyId: state.gatewayCredentials.accessKeyId,
                    secretAccessKey: state.gatewayCredentials.secretKey,
                    endpoint: state.gatewayCredentials.endpoint,
                    s3ForcePathStyle: true,
                    signatureVersion: 'v4',
                };

                state.s3Client = new AWS.S3(s3Config);
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
            clearObjects: function ({commit}: any): void {
                commit(CLEAR);
            },
        },
    };
}
