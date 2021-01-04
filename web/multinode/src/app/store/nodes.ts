// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { Node } from '@/nodes';

/**
 * NodesState is a representation of nodes module state.
 */
export class NodesState {
    public nodes: Node[] = [];
}

/**
 * NodesModule is a part of a global store that encapsulates all nodes related logic.
 */
export class NodesModule implements Module<NodesState, RootState> {
    public readonly namespaced: boolean;
    public readonly state: NodesState;
    public readonly getters?: GetterTree<NodesState, RootState>;
    public readonly actions: ActionTree<NodesState, RootState>;
    public readonly mutations: MutationTree<NodesState>;

    public constructor() { // here should be services, apis, 3d party dependencies.
        this.namespaced = true;

        this.state = new NodesState();

        this.mutations = {
            populate: this.populate,
        };

        this.actions = {
            fetch: this.fetch,
        };
    }

    /**
     * populate mutation will set state nodes with new nodes array.
     * @param state - state of the module.
     * @param nodes - nodes to save in state. all users nodes.
     */
    public populate(state: NodesState, nodes: Node[]): void {
        state.nodes = nodes;
    }

    /**
     * fetch action loads all nodes.
     * @param ctx - context of the Vuex action.
     */
    public async fetch(ctx: ActionContext<NodesState, RootState>): Promise<void> {
        await new Promise(() => null);
    }
}
