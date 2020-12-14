// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { projectsModule, paymentsModule }});

describe('OnboardingTourArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OnboardingTourArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
