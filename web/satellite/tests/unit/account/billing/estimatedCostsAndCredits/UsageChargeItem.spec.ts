// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import UsageChargeItem from '@/components/account/billing/estimatedCostsAndCredits/UsageChargeItem.vue';

import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { ProjectCharge } from '@/types/payments';
import { Project } from '@/types/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';

const localVue = createLocalVue();
localVue.filter('centsToDollars', (cents: number): string => {
    return `USD $${(cents / 100).toFixed(2)}`;
});
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const store = new Vuex.Store({ modules: { projectsModule, paymentsModule }});

describe('UsageChargeItem', () => {
    const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
    projectsApi.setMockProjects([project]);
    const date = new Date(Date.UTC(1970, 1, 1));
    const projectCharge = new ProjectCharge(date, date, 100, 100, 100, 'id', 100, 100, 100);

    it('renders correctly', () => {
        const wrapper = shallowMount(UsageChargeItem, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('toggling dropdown works correctly', async () => {
        const wrapper = shallowMount(UsageChargeItem, {
            store,
            localVue,
            propsData: {
                item: projectCharge,
            },
        });

        await wrapper.find('.usage-charge-item-container__summary').trigger('click');

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.usage-charge-item-container__summary').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
