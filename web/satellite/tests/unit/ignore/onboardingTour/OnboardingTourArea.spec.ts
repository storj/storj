// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '@/../tests/unit/mock/api/projects';
import { PaymentsHttpApi } from '@/api/payments';
import { router } from '@/router';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';

import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { projectsModule, paymentsModule } });

describe('OnboardingTourArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OnboardingTourArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
