// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import Vuex from 'vuex';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

import { APPSTATE_ACTIONS, appStateModule } from '@/app/store/modules/appState';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

Vue.directive('click-outside', {
    bind: (): void => { return; },
    unbind: (): void => { return; },
});

const store = new Vuex.Store({ modules: { appStateModule }});

describe('DiskStatChart', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(EstimationPeriodDropdown, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with calendar', async (): Promise<void> => {
        const wrapper = shallowMount(EstimationPeriodDropdown, {
            store,
            localVue,
        });

        await store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, true);

        expect(wrapper).toMatchSnapshot();
    });

    it('opens calendar on click',  (): void => {
        const wrapper = shallowMount(EstimationPeriodDropdown, {
            store,
            localVue,
        });

        wrapper.find('.period-container').trigger('click');

        expect(wrapper.find('.period-container__calendar').exists()).toBe(true);
    });
});
