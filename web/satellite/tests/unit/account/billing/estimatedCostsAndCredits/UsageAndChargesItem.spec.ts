// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';

import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { ProjectUsageAndCharges } from '@/types/payments';
import { Project } from '@/types/projects';
import { MetaUtils } from '@/utils/meta';

import UsageAndChargesItem from '@/components/account/billing/estimatedCostsAndCredits/UsageAndChargesItem.vue';

const localVue = createLocalVue();
localVue.filter('centsToDollars', (cents: number): string => {
    return `USD $${(cents / 100).toFixed(2)}`;
});
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const store = new Vuex.Store({ modules: { projectsModule, paymentsModule } });

jest.mock('@/utils/meta');

describe('UsageAndChargesItem', (): void => {
    const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
    projectsApi.setMockProjects([project]);
    const date = new Date(Date.UTC(1970, 1, 1));
    const projectCharge = new ProjectUsageAndCharges(date, date, 100, 100, 100, 'id', 100, 100, 100);

    MetaUtils.getMetaContent = jest.fn().mockReturnValue('1');

    it('renders correctly', (): void => {
        const wrapper = shallowMount(UsageAndChargesItem, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('toggling dropdown works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(UsageAndChargesItem, {
            store,
            localVue,
            propsData: {
                item: projectCharge,
            },
        });

        await wrapper.find('.usage-charges-item-container__summary').trigger('click');

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.usage-charges-item-container__summary').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
