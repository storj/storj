// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import CreditsHistory from '@/components/account/billing/freeCredits/CreditsHistory.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { router } from '@/router';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../mock/api/projects';

const localVue = createLocalVue();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const itemInvoice = new PaymentsHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Invoice);
const itemCharge = new PaymentsHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Charge);
const itemTransaction = new PaymentsHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const coupon = new PaymentsHistoryItem('testId', 'desc', 275, 0, 'test', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 275);
const coupon1 = new PaymentsHistoryItem('testId', 'desc', 500, 0, 'test', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 300);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);
const clickSpy = sinon.spy();

localVue.use(Vuex);
localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const store = new Vuex.Store({ modules: { paymentsModule, projectsModule }});
store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);
store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [itemInvoice, itemCharge, itemTransaction, coupon, coupon1]);

describe('CreditsHistory', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(CreditsHistory, {
            localVue,
            store,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on back works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(CreditsHistory, {
            localVue,
            store,
            router,
        });

        wrapper.vm.onBackToBillingClick = clickSpy;

        await wrapper.find('.credit-history__back-area').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
