// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { CreateNodeFields, Node, NodeURL } from '@/nodes';
import { Nodes } from '@/nodes/service';

/**
 * NodesState is a representation of nodes module state.
 */
export class NodesState {
    public nodes: Node[] = [];
    public selectedSatellite: NodeURL | null = null;
    public trustedSatellites: NodeURL[] = [];
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

    private readonly nodes: Nodes;

    public constructor(nodes: Nodes) {
        this.nodes = nodes;

        this.namespaced = true;
        this.state = new NodesState();
        this.mutations = {
            populate: this.populate,
            saveTrustedSatellites: this.saveTrustedSatellites,
            setSelectedSatellite: this.setSelectedSatellite,
        };
        this.actions = {
            fetch: this.fetch.bind(this),
            add: this.add.bind(this),
            trustedSatellites: this.trustedSatellites.bind(this),
            selectSatellite: this.selectSatellite.bind(this),
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
     * saveTrustedSatellites mutation will save new list of trusted satellites to store.
     * @param state
     * @param trustedSatellites
     */
    public saveTrustedSatellites(state: NodesState, trustedSatellites: NodeURL[]) {
        state.trustedSatellites = trustedSatellites;
    }

    /**
     * setSelectedSatellite mutation will selected satellite to store.
     * @param state
     * @param satelliteId - id of the satellite to select.
     */
    public setSelectedSatellite(state: NodesState, satelliteId: string) {
        state.selectedSatellite = state.trustedSatellites.find((satellite: NodeURL) => satellite.id === satelliteId) || null;
    }

    /**
     * fetch action loads all nodes information.
     * @param ctx - context of the Vuex action.
     */
    public async fetch(ctx: ActionContext<NodesState, RootState>): Promise<void> {
        const nodes = ctx.state.selectedSatellite ? await this.nodes.listBySatellite(ctx.state.selectedSatellite.id) : await this.nodes.list();
        ctx.commit('populate', nodes);
    }

    /**
     * Adds node to multinode list.
     * @param ctx - context of the Vuex action.
     * @param node - to add.
     */
    public async add(ctx: ActionContext<NodesState, RootState>, node: CreateNodeFields): Promise<void> {
        await this.nodes.add(node);
        await this.fetch(ctx);
    }

    /**
     * retrieves list of trusted satellites node urls for a node.
     * @param ctx - context of the Vuex action.
     */
    public async trustedSatellites(ctx: ActionContext<NodesState, RootState>): Promise<void> {
        const satellites: NodeURL[] = await this.nodes.trustedSatellites();

        ctx.commit('saveTrustedSatellites', satellites);
    }

    /**
     * save satellite as selected satellite.
     * @param ctx - context of the Vuex action.
     * @param satelliteId - satellite id to select.
     */
    public async selectSatellite(ctx: ActionContext<NodesState, RootState>, satelliteId: string): Promise<void> {
        ctx.commit('setSelectedSatellite', satelliteId);

        await this.fetch(ctx);
    }
}
