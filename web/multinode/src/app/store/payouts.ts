// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, GetterTree, Module, MutationTree } from 'vuex';

import { RootState } from '@/app/store/index';
import { monthNames } from '@/app/types/date';
import { Expectation, HeldAmountSummary, NodePayouts, PayoutsSummary, Paystub } from '@/payouts';
import { Payouts } from '@/payouts/service';

/**
 * PayoutsState is a representation of payouts module state.
 */
export class PayoutsState {
    public summary: PayoutsSummary = new PayoutsSummary();
    public selectedPayoutPeriod: string | null = null;
    public selectedNodePayouts: NodePayouts = new NodePayouts();
    public selectedNodeExpectations: Expectation = new Expectation();
    public totalExpectations: Expectation = new Expectation();
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

    private readonly payouts: Payouts;

    public constructor(payouts: Payouts) {
        this.payouts = payouts;

        this.namespaced = true;
        this.state = new PayoutsState();

        this.mutations = {
            setSummary: this.setSummary,
            setPayoutPeriod: this.setPayoutPeriod,
            setTotalExpectation: this.setTotalExpectation,
            setNodeTotals: this.setNodeTotals,
            setNodePaystub: this.setNodePaystub,
            setNodeHeldHistory: this.setNodeHeldHistory,
            setCurrentNodeExpectations: this.setCurrentNodeExpectations,
        };

        this.actions = {
            summary: this.summary.bind(this),
            expectations: this.expectations.bind(this),
            paystub: this.paystub.bind(this),
            nodeTotals: this.nodeTotals.bind(this),
            heldHistory: this.heldHistory.bind(this),
        };

        this.getters = {
            periodString: this.periodString,
        };
    }

    // Mutations
    /**
     * setSummary mutation will set payouts summary state.
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
    public setPayoutPeriod(state: PayoutsState, period: string | null): void {
        state.selectedPayoutPeriod = period;
    }

    /**
     * setTotalExpectation mutation will set total payouts expectation for all nodes.
     * @param state - state of the module.
     * @param expectations - payouts summary information depends on selected time and satellite.
     */
    public setTotalExpectation(state: PayoutsState, expectations: Expectation): void {
        state.totalExpectations = expectations;
    }

    /**
     * setNodeTotals mutation will total payout information for selected node to store.
     * @param state
     * @param paystub for all time
     */
    public setNodeTotals(state: PayoutsState, paystub: Paystub): void {
        state.selectedNodePayouts = { ...state.selectedNodePayouts, totalPaid: paystub.distributed, totalEarned: paystub.paid, totalHeld: paystub.held };
    }

    /**
     * setNodePaystub mutation will save paystub for selected node, satellite and period to store.
     * @param state
     * @param paystub
     */
    public setNodePaystub(state: PayoutsState, paystub: Paystub): void {
        state.selectedNodePayouts = { ...state.selectedNodePayouts, paystubForPeriod: paystub };
    }

    /**
     * setNodeHeldHistory mutation will save held history for selected node to store.
     * @param state
     * @param heldHistory
     */
    public setNodeHeldHistory(state: PayoutsState, heldHistory: HeldAmountSummary[]): void {
        state.selectedNodePayouts = { ...state.selectedNodePayouts, heldHistory };
    }

    /**
     * setCurrentNodeExpectations mutation will save paystub for selected node, satellite and period to store.
     * @param state
     * @param expectations
     */
    public setCurrentNodeExpectations(state: PayoutsState, expectations: Expectation): void {
        state.selectedNodePayouts = { ...state.selectedNodePayouts, expectations };
    }

    // Actions
    /**
     * summary action loads payouts summary information.
     * @param ctx - context of the Vuex action.
     */
    public async summary(ctx: ActionContext<PayoutsState, RootState>): Promise<void> {
        const selectedSatelliteId = ctx.rootState.nodes.selectedSatellite ? ctx.rootState.nodes.selectedSatellite.id : null;
        const summary = await this.payouts.summary(selectedSatelliteId, ctx.state.selectedPayoutPeriod);

        ctx.commit('setSummary', summary);
    }

    /**
     * expectations action loads payouts total or by node payout expectation information.
     * @param ctx - context of the Vuex action.
     * @param nodeId - node id.
     */
    public async expectations(ctx: ActionContext<PayoutsState, RootState>, nodeId: string | null): Promise<void> {
        const expectations = await this.payouts.expectations(nodeId);

        ctx.commit(`${nodeId ? 'setCurrentNodeExpectations' : 'setTotalExpectation'}`, expectations);
    }

    /**
     * paystub action loads payouts information for table for selected node.
     * @param ctx - context of the Vuex action.
     * @param nodeId
     */
    public async paystub(ctx: ActionContext<PayoutsState, RootState>, nodeId: string): Promise<void> {
        const selectedSatelliteId = ctx.rootState.nodes.selectedSatellite ? ctx.rootState.nodes.selectedSatellite.id : null;
        const paystub = await this.payouts.paystub(selectedSatelliteId, ctx.state.selectedPayoutPeriod, nodeId);

        ctx.commit('setNodePaystub', paystub);
    }

    /**
     * nodeTotals action loads total payouts information for selected node.
     * @param ctx - context of the Vuex action.
     * @param nodeId
     */
    public async nodeTotals(ctx: ActionContext<PayoutsState, RootState>, nodeId: string): Promise<void> {
        const paystub = await this.payouts.paystub(null, null, nodeId);

        ctx.commit('setNodeTotals', paystub);
    }

    /**
     * heldHistory action loads held history for selected node.
     * @param ctx - context of the Vuex action.
     * @param nodeId
     */
    public async heldHistory(ctx: ActionContext<PayoutsState, RootState>, nodeId: string): Promise<void> {
        const heldHistory = await this.payouts.heldHistory(nodeId);

        ctx.commit('setNodeHeldHistory', heldHistory);
    }

    // Getters
    /**
     * periodString is full name month and year representation of selected payout period.
     */
    public periodString(state: PayoutsState): string {
        if (!state.selectedPayoutPeriod) { return 'All time'; }

        const splittedPeriod = state.selectedPayoutPeriod.split('-');
        const monthIndex = parseInt(splittedPeriod[1]) - 1;
        const year = splittedPeriod[0];

        return `${monthNames[monthIndex]}, ${year}`;
    }
}
