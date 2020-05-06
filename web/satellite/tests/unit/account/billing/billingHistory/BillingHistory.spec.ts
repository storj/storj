// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import BillingHistory from '@/components/account/billing/billingHistory/BillingHistory.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { BillingHistoryItem, BillingHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../mock/api/projects';

const localVue = createLocalVue();
const segmentioPlugin = new SegmentioPlugin();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const itemInvoice = new BillingHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Invoice);
const itemCharge = new BillingHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Charge);
const itemTransaction = new BillingHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Transaction);
const itemTransaction1 = new BillingHistoryItem('testId3', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Transaction);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);
const clickSpy = sinon.spy();

localVue.use(Vuex);
localVue.use(segmentioPlugin);

const store = new Vuex.Store({ modules: { paymentsModule, projectsModule }});
store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [itemCharge, itemInvoice, itemTransaction, itemTransaction1]);
store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

describe('BillingHistory', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(BillingHistory, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on back works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(BillingHistory, {
            localVue,
            store,
            methods: {
                onBackToAccountClick: clickSpy,
            },
        });

        await wrapper.find('.billing-history-area__title-area').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
