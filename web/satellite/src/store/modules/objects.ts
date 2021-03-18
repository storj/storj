// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { GatewayCredentials } from '@/types/accessGrants';

export const OBJECTS_ACTIONS = {
    CLEAR: 'clearObjects',
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
};

export const OBJECTS_MUTATIONS = {
    SET_GATEWAY_CREDENTIALS: 'setGatewayCredentials',
    SET_ACCESS_GRANT: 'setAccessGrant',
    CLEAR: 'clearObjects',
};

const {
    CLEAR,
    SET_ACCESS_GRANT,
    SET_GATEWAY_CREDENTIALS,
} = OBJECTS_MUTATIONS;

export class ObjectsState {
    public accessGrant: string = '';
    public gatewayCredentials: GatewayCredentials = new GatewayCredentials();
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
            clearObjects: function ({commit}: any): void {
                commit(CLEAR);
            },
        },
    };
}
