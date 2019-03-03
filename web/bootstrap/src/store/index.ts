// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { ACTIONS, MUTATIONS } from '@/utils/constants';
import { checkAvailability } from '@/api/bootstrap';
import { NodeStatus } from '@/types/nodeStatus';

Vue.use(Vuex);

// Bootstrap store (vuex)
const store = new Vuex.Store({
    state: {
        isLoading: false,
        nodeStatus: 0,
    },
    mutations: {
        [MUTATIONS.SET_NODE_STATUS](state: any, status: NodeStatus): void {
            state.isLoading = true;
            state.nodeStatus = status;
        },
    },
    actions: {
        async [ACTIONS.CHECK_NODE_STATUS]({commit}: any, nodeId: string): Promise<any> {
            let nodeStatus = await checkAvailability(nodeId);

            commit(MUTATIONS.SET_NODE_STATUS, nodeStatus);
        },
    },
});

export default store;
