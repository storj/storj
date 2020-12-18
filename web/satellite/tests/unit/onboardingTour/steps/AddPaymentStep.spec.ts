// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AddPaymentStep from '@/components/onboardingTour/steps/AddPaymentStep.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

const localVue = createLocalVue();
localVue.use(Vuex);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { projectsModule, paymentsModule }});

describe('AddPaymentStep.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(AddPaymentStep, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.payment-step__methods-container__title-area__options-area__token').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
