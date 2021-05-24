// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { monthNames } from '@/app/types/date';
import { PayoutsSummary } from '@/payouts';
import { Payouts } from '@/payouts/service';

/**
 * PayoutsState is a representation of payouts module state.
 */
export class PayoutsState {
    public summary: PayoutsSummary = new PayoutsSummary();
    public selectedPayoutPeriod: string | null = null;
}

/**
 * PayoutsModule is a part of a global store that encapsulates all payouts related logic.
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
            setSummary: this.setSummary,
            setPayoutPeriod: this.setPayoutPeriod,
        };
        this.actions = {
            summary: this.summary.bind(this),
        };
        this.getters = {
            periodString: this.periodString,
        };
    }

    // Mutations
    /**
     * populate mutation will set payouts state.
     * @param state - state of the module.
     * @param summary - payouts summary information depends on selected time and satellite.
     */
    public setSummary(state: PayoutsState, summary: PayoutsSummary): void {
        state.summary = summary;
    }

    /**
     * setPayoutPeriod mutation will save selected period to store.
     * @param state
     * @param period representation of month and year
     */
    public setPayoutPeriod(state: PayoutsState, period: string | null) {
        state.selectedPayoutPeriod = period;
    }

    // Actions
    /**
     * summary action loads payouts summary information.
     * @param ctx - context of the Vuex action.
     */
    public async summary(ctx: ActionContext<PayoutsState, RootState>): Promise<void> {
        // @ts-ignore
        const selectedSatelliteId = ctx.rootState.nodes.selectedSatellite ? ctx.rootState.nodes.selectedSatellite.id : null;
        const summary = await this.payouts.summary(selectedSatelliteId, ctx.state.selectedPayoutPeriod);

        ctx.commit('setSummary', summary);
    }

    // Getters
    /**
     * periodString is full name month and year representation of selected payout period.
     */
    public periodString(state: PayoutsState): string {
        if (!state.selectedPayoutPeriod) return 'All time';

        const splittedPeriod = state.selectedPayoutPeriod.split('-');
        const monthIndex = parseInt(splittedPeriod[1]) - 1;
        const year = splittedPeriod[0];

        return `${monthNames[monthIndex]}, ${year}`;
    }
}
