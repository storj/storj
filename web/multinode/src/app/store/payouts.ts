// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { PayoutsSummary } from '@/payouts';
import { Payouts } from '@/payouts/service';

/**
 * PayoutsState is a representation of payouts module state.
 */
export class PayoutsState {
    public summary: PayoutsSummary;
}

/**
 * NodesModule is a part of a global store that encapsulates all nodes related logic.
 */
export class PayoutsModule implements Module<PayoutsState, RootState> {
    public readonly namespaced: boolean;
    public readonly state: PayoutsState;
    public readonly getters?: GetterTree<PayoutsState, RootState>;
    public readonly actions: ActionTree<PayoutsState, RootState>;
    public readonly mutations: MutationTree<PayoutsState>;

    private readonly payouts: any;

    public constructor(payouts: Payouts) {
        this.payouts = payouts;

        this.namespaced = true;
        this.state = new PayoutsState();
        this.mutations = {
            populate: this.populate,
        };
        this.actions = {
            getSummary: this.getSummary.bind(this),
        };
    }

    /**
     * populate mutation will set payouts state.
     * @param state - state of the module.
     * @param summary - payouts summary information depends on selected time and satellite.
     */
    public populate(state: PayoutsState, summary: PayoutsSummary): void {
        state.summary = summary;
    }

    /**
     * getSummary action loads payouts summary information.
     * @param ctx - context of the Vuex action.
     */
    public async getSummary(ctx: ActionContext<PayoutsState, RootState>): Promise<void> {
        // @ts-ignore
        const summary = await this.payouts.summary(ctx.rootState.nodes.selectedSatellite.id, '');
        ctx.commit('populate', summary);
    }
}
