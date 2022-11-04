// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { NodePayoutsSummary } from '@/payouts';

import PayoutsSummaryTable from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryTable.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('PayoutsSummaryTable', (): void => {
    it('renders correctly', (): void => {
        const nodePayoutsSummary = [
            new NodePayoutsSummary('1', 'name1', 5000, 4000),
            new NodePayoutsSummary('2', 'name2', 500, 1000),
        ];

        const wrapper = shallowMount(PayoutsSummaryTable, {
            localVue,
            propsData: { nodePayoutsSummary },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
