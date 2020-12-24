// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex, { ModuleTree, Store, StoreOptions } from 'vuex';

import { NodesModule, NodesState } from '@/app/store/nodes';

Vue.use(Vuex); // TODO: place to main.ts when initialization of everything will be there.

/**
 * RootState is a representation of global state.
 */
export class RootState {
    nodes: NodesState;
}

/**
 * MultinodeStoreOptions contains all needed data for store creation.
 */
class MultinodeStoreOptions implements StoreOptions<RootState> {
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

export const store: Store<RootState> = new Vuex.Store<RootState>(
    new MultinodeStoreOptions(new NodesModule()),
);
