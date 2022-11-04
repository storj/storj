// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import store from '../../mock/store';

import { Currency } from '@/app/utils/currency';
import PayoutsByNode from '@/app/views/payouts/PayoutsByNode.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

const $route = {
    fullPath: '/payouts/by-node/1',
    params: { id: '1' },
};

describe('PayoutsByNode', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayoutsByNode, {
            store,
            localVue,
            mocks: {
                $route,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
