// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';
import { UsersApiMock } from '../../../mock/api/users';

import { appStateModule } from '@/store/modules/appState';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { NOTIFICATION_MUTATIONS } from '@/store/mutationConstants';
import { PaymentsHistoryItem, PaymentsHistoryItemStatus, PaymentsHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { NotificatorPlugin } from '@/utils/plugins/notificator';

import AddStorjForm from '@/components/account/billing/paymentMethods/AddStorjForm.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();

const store = new Vuex.Store<{
    usersModule: typeof usersModule.state,
    paymentsModule: typeof paymentsModule.state,
    projectsModule: typeof projectsModule.state,
    appStateModule: typeof appStateModule.state,
    notificationsModule: typeof notificationsModule.state,
}>({ modules: { usersModule, paymentsModule, projectsModule, appStateModule, notificationsModule } });
store.commit(USER_MUTATIONS.SET_USER, new User('id', 'name', 'short', 'test@test.test', 'partner', 'pass'));

localVue.use(new NotificatorPlugin(store));

describe('AddStorjForm', () => {
    it('renders correctly', () => {
        const wrapper = mount<AddStorjForm>(AddStorjForm, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('user is unable to add less than 10$ or more than 999999$ for the first time', async () => {
        const wrapper = mount<AddStorjForm>(AddStorjForm, {
            store,
            localVue,
        });

        wrapper.vm.$data.tokenDepositValue = 5;
        await wrapper.vm.onConfirmAddSTORJ();

        expect(store.state.notificationsModule.notificationQueue[0].message).toMatch('First deposit amount must be more than $10 and less than $1000000');

        wrapper.vm.$data.tokenDepositValue = 1000000;
        await wrapper.vm.onConfirmAddSTORJ();

        expect(store.state.notificationsModule.notificationQueue[1].message).toMatch('First deposit amount must be more than $10 and less than $1000000');
    });

    it('user is able to add less than 10$ after coupon is applied', async () => {
        window.open = jest.fn();
        const billingTransactionItem = new PaymentsHistoryItem('itemId', 'test', 10, 10,
            PaymentsHistoryItemStatus.Completed, 'test', new Date(), new Date(), PaymentsHistoryItemType.Transaction);
        const project = new Project('testId', 'test', 'test', 'test', 'id', true);
        store.commit(NOTIFICATION_MUTATIONS.CLEAR);
        store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [billingTransactionItem]);
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        const wrapper = mount<AddStorjForm>(AddStorjForm, {
            store,
            localVue,
        });

        wrapper.vm.$data.tokenDepositValue = 5;
        await wrapper.vm.onConfirmAddSTORJ();

        expect(store.state.notificationsModule.notificationQueue[0].type).toMatch('SUCCESS');
    });

    it('renders correctly after continue To Coin Payments click', () => {
        window.open = jest.fn();
        const wrapper = shallowMount<AddStorjForm>(AddStorjForm, {
            store,
            localVue,
            propsData: {
                isLoading: true,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
