// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../mock/api/projects';

import { PaymentsHttpApi } from '@/api/payments';
import { router } from '@/router';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';

import DetailedHistory from '@/components/account/billing/depositAndBillingHistory/DetailedHistory.vue';

const localVue = createLocalVue();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const notificationsModule = makeNotificationsModule();
const itemInvoice = new PaymentsHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Invoice);
const itemCharge = new PaymentsHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Charge);
const itemTransaction = new PaymentsHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const itemTransaction1 = new PaymentsHistoryItem('testId3', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);

localVue.use(Vuex);

const store = new Vuex.Store({ modules: { paymentsModule, projectsModule, notificationsModule } });
store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

localVue.use(new NotificatorPlugin(store));

describe('DetailedHistory', (): void => {
    it('renders correctly without items', async (): Promise<void> => {
        const wrapper = shallowMount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        await wrapper.setData({ isDataFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with deposit items', async (): Promise<void> => {
        await store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [itemTransaction, itemTransaction1, itemInvoice, itemCharge]);

        const wrapper = shallowMount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        await wrapper.setData({ isDataFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on back works correctly', async (): Promise<void> => {
        const clickSpy = jest.spyOn(router, 'push');
        const wrapper = mount(DetailedHistory, {
            localVue,
            store,
            router,
        });

        await wrapper.find('.history-area__back-area').trigger('click');
        expect(clickSpy).toHaveBeenCalled();
    });
});
