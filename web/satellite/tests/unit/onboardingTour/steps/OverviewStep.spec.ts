// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

import { router } from '@/router';
import { appStateModule } from "@/store/modules/appState";
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: { appStateModule }});

describe('OverviewStep.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(OverviewStep, {
            localVue,
            router,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
