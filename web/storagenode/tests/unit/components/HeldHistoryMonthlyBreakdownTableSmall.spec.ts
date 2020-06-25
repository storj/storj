// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import HeldHistoryMonthlyBreakdownTableSmall from '@/app/components/payments/HeldHistoryMonthlyBreakdownTableSmall.vue';

import { HeldHistoryMonthlyBreakdownItem } from '@/app/types/payout';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

describe('HeldHistoryMonthlyBreakdownTableSmall', (): void => {
    it('renders correctly with actual values',  async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryMonthlyBreakdownTableSmall, {
            propsData: {
                heldHistoryItem: new HeldHistoryMonthlyBreakdownItem(
                    '1',
                    'name1',
                    6,
                    50000,
                    7333880,
                    7852235,
                    0,
                ),
            },
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.find('.expand').trigger('click');

        await localVue.nextTick();

        expect(wrapper).toMatchSnapshot();

        wrapper.find('.hide').trigger('click');

        await localVue.nextTick();

        expect(wrapper).toMatchSnapshot();
    });
});
