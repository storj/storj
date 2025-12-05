// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import { Expectation, NodePayouts, PayoutsSummary } from '@/payouts';
import { Payouts } from '@/payouts/service';
import { PayoutsClient } from '@/api/payouts';
import { useNodesStore } from '@/app/store/pinia/nodesStore';
import { monthNames } from '@/app/types/date';

class PayoutsState {
    public summary: PayoutsSummary = new PayoutsSummary();
    public selectedPayoutPeriod: string | null = null;
    public selectedNodePayouts: NodePayouts = new NodePayouts();
    public selectedNodeExpectations: Expectation = new Expectation();
    public totalExpectations: Expectation = new Expectation();
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const usePayoutsStore = defineStore('payouts', () => {
    const state = reactive(new PayoutsState());

    const service = new Payouts(new PayoutsClient());

    const nodesStore = useNodesStore();

    const periodString = computed<string>(() => {
        if (!state.selectedPayoutPeriod) { return 'All time'; }

        const splittedPeriod = state.selectedPayoutPeriod.split('-');
        const monthIndex = parseInt(splittedPeriod[1]) - 1;
        const year = splittedPeriod[0];

        return `${monthNames[monthIndex]}, ${year}`;
    });

    async function summary(): Promise<void> {
        const selectedSatelliteId = nodesStore.state.selectedSatellite ? nodesStore.state.selectedSatellite.id : null;
        state.summary = await service.summary(selectedSatelliteId, state.selectedPayoutPeriod);
    }

    async function expectations(nodeID: string | null): Promise<void> {
        const expectations = await service.expectations(nodeID);

        if (nodeID) {
            state.selectedNodePayouts = { ...state.selectedNodePayouts, expectations };
        } else {
            state.totalExpectations = expectations;
        }
    }

    async function paystub(nodeID: string): Promise<void> {
        const selectedSatelliteId = nodesStore.state.selectedSatellite ? nodesStore.state.selectedSatellite.id : null;
        const paystub = await service.paystub(selectedSatelliteId, state.selectedPayoutPeriod, nodeID);

        state.selectedNodePayouts = { ...state.selectedNodePayouts, paystubForPeriod: paystub };
    }

    async function nodeTotals(nodeID: string): Promise<void> {
        const paystub = await service.paystub(null, null, nodeID);

        state.selectedNodePayouts = { ...state.selectedNodePayouts, totalPaid: paystub.distributed, totalEarned: paystub.paid, totalHeld: paystub.held };
    }

    async function heldHistory(nodeID: string): Promise<void> {
        const heldHistory = await service.heldHistory(nodeID);

        state.selectedNodePayouts = { ...state.selectedNodePayouts, heldHistory };
    }

    return {
        state,
        periodString,
        summary,
        expectations,
        paystub,
        nodeTotals,
        heldHistory,
    };
});
