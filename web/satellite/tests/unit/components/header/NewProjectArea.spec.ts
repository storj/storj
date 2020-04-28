// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NewProjectArea from '@/components/header/NewProjectArea.vue';

import { router } from '@/router';
import { appStateModule } from '@/store/modules/appState';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { BillingHistoryItem, BillingHistoryItemStatus, BillingHistoryItemType, CreditCard } from '@/types/payments';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsMock } from '../../mock/api/payments';
import { ProjectsApiMock } from '../../mock/api/projects';
import { UsersApiMock } from '../../mock/api/users';

const localVue = createLocalVue();
localVue.use(Vuex);

const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { usersModule, projectsModule, paymentsModule, appStateModule }});

describe('NewProjectArea', () => {
    it('renders correctly without projects and without payment methods', () => {
        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.new-project-button-container').length).toBe(0); // user is unable to create project.
    });

    it('renders correctly without projects and with credit card', async () => {
        const creditCard = new CreditCard('id', 1, 2000, 'test', '0000', true);

        await store.commit(PAYMENTS_MUTATIONS.SET_CREDIT_CARDS, [creditCard]);

        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.new-project-button-container').length).toBe(1);

        await wrapper.find('.new-project-button-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly without projects and with completed 50$ transaction', () => {
        const billingTransactionItem = new BillingHistoryItem('itemId', 'test', 50, 50,
            BillingHistoryItemStatus.Completed, 'test', new Date(), new Date(), BillingHistoryItemType.Transaction);
        store.commit(PAYMENTS_MUTATIONS.CLEAR);
        store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [billingTransactionItem]);
        store.commit(PAYMENTS_MUTATIONS.SET_BALANCE, 50);

        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper.findAll('.new-project-button-container').length).toBe(1);
    });

    it('user is unable to create project with his project and with payment method', () => {
        const user = new User('ownerId', 'test', 'test', 'test@test.test', 'test', 'test');
        const project = new Project('id', 'test', 'test', 'test', 'ownerId', true);
        store.commit(APP_STATE_MUTATIONS.TOGGLE_NEW_PROJECT_POPUP);
        store.commit(USER_MUTATIONS.SET_USER, user);
        store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);

        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper.findAll('.new-project-button-container').length).toBe(0);
    });
});
