// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayoutsPage from '@/app/views/PayoutsPage.vue';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import store from '../../mock/store';

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
