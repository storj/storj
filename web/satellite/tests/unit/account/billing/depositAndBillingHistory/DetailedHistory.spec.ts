// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import DetailedHistory from '@/components/account/billing/depositAndBillingHistory/DetailedHistory.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { router } from '@/router';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../mock/api/projects';

const localVue = createLocalVue();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const itemInvoice = new PaymentsHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Invoice);
const itemCharge = new PaymentsHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Charge);
const itemTransaction = new PaymentsHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const itemTransaction1 = new PaymentsHistoryItem('testId3', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);

localVue.use(Vuex);

const store = new Vuex.Store({ modules: { paymentsModule, projectsModule }});
store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

describe('DetailedHistory', (): void => {
    it('renders correctly without items', (): void => {
        const wrapper = shallowMount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with deposit items', (): void => {
        store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [itemTransaction, itemTransaction1, itemInvoice, itemCharge]);

        const wrapper = shallowMount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on back works correctly', async (): Promise<void> => {
        const clickSpy = sinon.spy();
        const wrapper = mount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        wrapper.vm.onBackToBillingClick = clickSpy;

        await wrapper.find('.history-area__back-area').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
