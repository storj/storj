// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex, { ModuleTree, Store, StoreOptions } from 'vuex';

import { NodesClient } from '@/api/nodes';
import { PayoutsClient } from '@/api/payouts';
import { NodesModule, NodesState } from '@/app/store/nodes';
import { PayoutsModule, PayoutsState } from '@/app/store/payouts';
import { Nodes } from '@/nodes/service';
import { Payouts } from '@/payouts/service';

Vue.use(Vuex);

/**
 * RootState is a representation of global state.
 */
export class RootState {
    nodes: NodesState;
    payouts: PayoutsState;
}

/**
 * MultinodeStoreOptions contains all needed data for store creation.
 */
export class MultinodeStoreOptions implements StoreOptions<RootState> {
    public readonly strict: boolean;
    public readonly state: RootState;
    public readonly modules: ModuleTree<RootState>;

    public constructor(nodes: NodesModule, payouts: PayoutsModule) {
        this.strict = true;
        this.state = {
            nodes: nodes.state,
            payouts: payouts.state,
        };
        this.modules = {
            nodes,
            payouts,
        };
    }
}

// Services
const nodesClient: NodesClient = new NodesClient();
const nodesService: Nodes = new Nodes(nodesClient);
const payoutsClient: PayoutsClient = new PayoutsClient();
const payoutsService: Payouts = new Payouts(payoutsClient);

// Modules
const nodesModule: NodesModule = new NodesModule(nodesService);
const payoutsModule: PayoutsModule = new PayoutsModule(payoutsService);

// Store
export const store: Store<RootState> = new Vuex.Store<RootState>(
    new MultinodeStoreOptions(nodesModule, payoutsModule),
);
