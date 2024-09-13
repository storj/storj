// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { BandwidthTraffic } from '@/bandwidth';
import { Bandwidth } from '@/bandwidth/service';
import { NodeStatus } from '@/nodes';

/**
 * BandwidthState is a representation of bandwidth egress and ingress.
 */
export class BandwidthState {
    public traffic: BandwidthTraffic = new BandwidthTraffic();
}

/**
 * BandwidthModule is a part of a global store that encapsulates all bandwidth related logic.
 */
export class BandwidthModule implements Module<BandwidthState, RootState> {
    public readonly namespaced: boolean;
    public readonly state: BandwidthState;
    public readonly getters?: GetterTree<BandwidthState, RootState>;
    public readonly actions: ActionTree<BandwidthState, RootState>;
    public readonly mutations: MutationTree<BandwidthState>;

    private readonly bandwidth: Bandwidth;

    public constructor(bandwidth: Bandwidth) {
        this.bandwidth = bandwidth;

        this.namespaced = true;
        this.state = new BandwidthState();

        this.mutations = {
            populate: this.populate,
        };

        this.actions = {
            fetch: this.fetch.bind(this),
        };
    }

    /**
     * populate mutation will set bandwidth state.
     * @param state - state of the module.
     * @param traffic - representation of bandwidth egress and ingress.
     */
    public populate(state: BandwidthState, traffic: BandwidthTraffic): void {
        state.traffic = traffic;
    }

    /**
     * fetch action loads bandwidth information.
     * @param ctx - context of the Vuex action.
     */
    public async fetch(ctx: ActionContext<BandwidthState, RootState>): Promise<void> {
        const selectedSatelliteId = ctx.rootState.nodes.selectedSatellite ? ctx.rootState.nodes.selectedSatellite.id : ctx.rootState.nodes.trustedSatellites[0].id;
        const selectedNodeId = ctx.rootState.nodes.selectedNode ? ctx.rootState.nodes.selectedNode.id : null;
        const traffic = await this.bandwidth.fetch(selectedSatelliteId, selectedNodeId);

        ctx.commit('populate', traffic);
    }
}
