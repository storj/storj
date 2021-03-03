// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex, { ModuleTree, Store, StoreOptions } from 'vuex';

import { NodesClient } from '@/api/nodes';
import { NodesModule, NodesState } from '@/app/store/nodes';
import { Nodes } from '@/nodes/service';

Vue.use(Vuex);

/**
 * RootState is a representation of global state.
 */
export class RootState {
    nodes: NodesState;
}

/**
 * MultinodeStoreOptions contains all needed data for store creation.
 */
export class MultinodeStoreOptions implements StoreOptions<RootState> {
    public readonly strict: boolean;
    public readonly state: RootState;
    public readonly modules: ModuleTree<RootState>;

    public constructor(nodes: NodesModule) {
        this.strict = true;
        this.state = {
            nodes: nodes.state,
        };
        this.modules = {
            nodes,
        };
    }
}

// Services
const nodesClient: NodesClient = new NodesClient();
const nodesService: Nodes = new Nodes(nodesClient);

// Modules
const nodesModule: NodesModule = new NodesModule(nodesService);

// Store
export const store: Store<RootState> = new Vuex.Store<RootState>(
    new MultinodeStoreOptions(nodesModule),
);
