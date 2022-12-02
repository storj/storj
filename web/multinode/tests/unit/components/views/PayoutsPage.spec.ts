// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import store from '../../mock/store';

import PayoutsPage from '@/app/views/payouts/PayoutsPage.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('PayoutsPage', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayoutsPage, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
